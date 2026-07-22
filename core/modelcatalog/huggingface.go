package modelcatalog

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type HuggingFaceCatalog struct {
	client          *http.Client
	modelStorageDir string
	baseURL         string
	cache           *catalogCache
	broker          *refreshBroker
	mu              sync.Mutex
	refreshing      map[string]struct{}
}

type HuggingFaceCatalogOptions struct {
	HTTPClient      *http.Client
	ModelStorageDir string
	BaseURL         string
	CacheDir        string
	CacheTTL        time.Duration
}

func NewHuggingFaceCatalog(opts HuggingFaceCatalogOptions) *HuggingFaceCatalog {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL := cmp.Or(opts.BaseURL, "https://huggingface.co")
	return &HuggingFaceCatalog{
		client:          client,
		modelStorageDir: filepath.Clean(opts.ModelStorageDir),
		baseURL:         strings.TrimRight(baseURL, "/"),
		cache:           newCatalogCache(opts.CacheDir, opts.CacheTTL),
		broker:          newRefreshBroker(),
		refreshing:      map[string]struct{}{},
	}
}

type modelInfo struct {
	ID, ModelID, LastModified string
	Downloads, Likes          int64
	Tags                      []string
	Config                    struct {
		Architectures    []string
		ModelType        string `json:"model_type"`
		NumExpertsPerTok int64  `json:"num_experts_per_tok"`
	}
	Safetensors struct{ Total int64 }
	GGUF        modelGGUF `json:"gguf"`
	Siblings    []sibling
}

type modelGGUF struct {
	Total         int64
	Architecture  string `json:"architecture"`
	ContextLength int64  `json:"context_length"`
}

type listedModel struct {
	ID           string   `json:"id"`
	ModelID      string   `json:"modelId"`
	Downloads    int64    `json:"downloads"`
	Likes        int64    `json:"likes"`
	LastModified string   `json:"lastModified"`
	Tags         []string `json:"tags"`
}

type sibling struct {
	RFilename string   `json:"rfilename"`
	Size      int64    `json:"size"`
	LFS       *lfsInfo `json:"lfs"`
}

type lfsInfo struct {
	Size int64 `json:"size"`
}

func (c *HuggingFaceCatalog) Resolve(ctx context.Context, rawURL string) (Resolution, error) {
	source, err := ParseHuggingFaceURL(rawURL)
	if err != nil {
		return Resolution{}, err
	}
	info, _, err := c.fetchModelInfoRaw(ctx, source)
	if err != nil {
		return Resolution{}, err
	}
	files, err := c.filesFor(source, info.Siblings)
	if err != nil {
		return Resolution{}, err
	}
	if len(files) == 0 {
		return Resolution{}, fmt.Errorf("%w: expected a Hugging Face model URL containing downloadable .gguf files", ErrInvalidInput)
	}
	params, arch, ctxLen, moe := modelMetadata(info)
	description := c.fetchDescription(ctx, source)
	return Resolution{OK: true, Source: source, Description: description, Params: params, Architecture: arch, ContextLength: ctxLen, IsMoE: moe, LlamaCPP: LlamaCPPCompatibility{Compatible: true, HFRef: source.Owner + "/" + source.Repo}, Files: files}, nil
}

func (c *HuggingFaceCatalog) Subscribe() (<-chan RefreshEvent, func()) {
	return c.broker.Subscribe()
}

func (c *HuggingFaceCatalog) List(ctx context.Context, req ListRequest, machine MachineProfile) (ListResult, error) {
	params := normalizeListRequest(req)
	var cacheErr error
	if c.cache != nil {
		if entry, ok, err := c.cache.load(params, machine); err == nil && ok {
			result := refitResult(entry.Result, machine)
			result.Cache = c.cache.state(entry)
			result.Cache.Refreshing = result.Cache.Stale
			if result.Cache.Stale {
				c.refreshAsync(params, machine)
			}
			return result, nil
		} else if err != nil {
			cacheErr = fmt.Errorf("read catalog cache: %w", err)
		}
	}
	result, rawList, rawModels, err := c.fetchCatalog(ctx, params, machine)
	if err != nil {
		if cacheErr != nil {
			return ListResult{}, errors.Join(cacheErr, err)
		}
		return ListResult{}, err
	}
	if c.cache != nil {
		if err := c.cache.store(params, machine, result, rawList, rawModels); err != nil {
			return ListResult{}, err
		}
	}
	return result, nil
}

func (c *HuggingFaceCatalog) refreshAsync(params normalizedParams, machine MachineProfile) {
	if c.cache == nil {
		return
	}
	key := c.cache.path(params)
	c.mu.Lock()
	if _, ok := c.refreshing[key]; ok {
		c.mu.Unlock()
		return
	}
	c.refreshing[key] = struct{}{}
	c.mu.Unlock()
	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.refreshing, key)
			c.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		result, rawList, rawModels, err := c.fetchCatalog(ctx, params, machine)
		event := RefreshEvent{Type: "catalog_refresh", OK: err == nil, Search: params.Search, Sort: params.Sort, MinFit: params.MinFit}
		if err == nil {
			err = c.cache.store(params, machine, result, rawList, rawModels)
		}
		if err != nil {
			event.OK = false
			event.Error = err.Error()
		}
		c.broker.publish(event)
	}()
}

func refitResult(result ListResult, machine MachineProfile) ListResult {
	result.Machine = machine
	for i := range result.Models {
		model := &result.Models[i]
		for j := range model.Files {
			model.Files[j] = estimateFileFit(model.Files[j], machine)
		}
		if len(model.Files) == 0 {
			model.BestFile = nil
			model.Fit = estimateFit(0, machine)
			continue
		}
		best := bestFile(model.Files)
		model.BestFile = &best
		model.Fit = estimateFit(best.EstimatedRAMBytes, machine)
		model.Score = scoreModel(*model)
	}
	sortCatalogModels(result.Models)
	return result
}

// sortCatalogModels orders models by descending score, tie-broken by ID.
func sortCatalogModels(models []CatalogModel) {
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Score != models[j].Score {
			return models[i].Score > models[j].Score
		}
		return models[i].ID < models[j].ID
	})
}

func (c *HuggingFaceCatalog) fetchCatalog(ctx context.Context, params normalizedParams, machine MachineProfile) (ListResult, json.RawMessage, map[string]json.RawMessage, error) {
	listed, rawList, err := c.fetchTopModels(ctx, params)
	if err != nil {
		return ListResult{}, nil, nil, err
	}
	rawModels := map[string]json.RawMessage{}
	models := make([]CatalogModel, 0, len(listed))
	failures := []string{}
	for _, item := range listed {
		model, rawKey, rawInfo, ok, err := c.catalogModel(ctx, item, params, machine)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", cmp.Or(item.ID, item.ModelID), err))
			continue
		}
		if !ok {
			continue
		}
		rawModels[rawKey] = rawInfo
		models = append(models, model)
	}
	sortCatalogModels(models)
	result := ListResult{OK: true, Machine: machine, Models: models, Cache: CacheState{Hit: false, TTLSeconds: int64(c.cacheTTL().Seconds())}, Errors: failures, Params: params}
	return result, rawList, rawModels, nil
}

func (c *HuggingFaceCatalog) catalogModel(ctx context.Context, item listedModel, params normalizedParams, machine MachineProfile) (CatalogModel, string, json.RawMessage, bool, error) {
	source, ok := sourceFromModelID(cmp.Or(item.ID, item.ModelID))
	if !ok {
		return CatalogModel{}, "", nil, false, nil
	}
	info, rawInfo, err := c.fetchModelInfoRaw(ctx, source)
	if err != nil {
		return CatalogModel{}, "", nil, false, err
	}
	files, err := c.catalogFiles(source, info.Siblings, params, machine)
	if err != nil {
		return CatalogModel{}, "", nil, false, err
	}
	if len(files) == 0 {
		return CatalogModel{}, "", nil, false, nil
	}
	best := bestFile(files)
	model := buildCatalogModel(source, item, info, files, best, machine)
	return model, source.Owner + "/" + source.Repo, rawInfo, true, nil
}

func (c *HuggingFaceCatalog) catalogFiles(source Source, siblings []sibling, params normalizedParams, machine MachineProfile) ([]File, error) {
	files, err := c.filesFor(source, siblings)
	if err != nil {
		return nil, err
	}
	for i := range files {
		files[i] = estimateFileFit(files[i], machine)
	}
	if len(files) == 0 {
		return nil, nil
	}
	best := bestFile(files)
	if !passesMinFit(best.FitLevel, params.MinFit) || (params.LocalOnly && !best.Exists) {
		return nil, nil
	}
	return files, nil
}

func buildCatalogModel(source Source, item listedModel, info modelInfo, files []File, best File, machine MachineProfile) CatalogModel {
	tags := mergeTags(item.Tags, info.Tags)
	params, arch, ctxLen, moe := modelMetadata(info)
	model := CatalogModel{ID: source.Owner + "/" + source.Repo, Owner: source.Owner, Repo: source.Repo, URL: source.URL, Downloads: cmp.Or(info.Downloads, item.Downloads), Likes: cmp.Or(info.Likes, item.Likes), LastModified: cmp.Or(info.LastModified, item.LastModified), Tags: tags, License: licenseFromTags(tags), Params: params, Architecture: arch, ContextLength: ctxLen, IsMoE: moe, Files: files, BestFile: &best, Fit: estimateFit(best.EstimatedRAMBytes, machine)}
	model.Score = scoreModel(model)
	return model
}

func modelMetadata(info modelInfo) (int64, string, int64, bool) {
	if info.GGUF.Total > 0 || info.GGUF.Architecture != "" || info.GGUF.ContextLength > 0 {
		arch := strings.TrimSpace(info.GGUF.Architecture)
		return info.GGUF.Total, arch, info.GGUF.ContextLength, isMoE(arch, info.Config.NumExpertsPerTok)
	}
	candidates := make([]string, 0, len(info.Config.Architectures)+1)
	for _, v := range info.Config.Architectures {
		candidates = append(candidates, strings.TrimSpace(v))
	}
	candidates = append(candidates, strings.TrimSpace(info.Config.ModelType))
	arch := cmp.Or(candidates...)
	return info.Safetensors.Total, arch, 0, isMoE(arch, info.Config.NumExpertsPerTok)
}

func isMoE(arch string, numExpertsPerTok int64) bool {
	if numExpertsPerTok > 1 {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(arch))
	switch value {
	case "deepseek2", "mixtral", "dbrx", "jamba", "arctic", "grok":
		return true
	default:
		return strings.Contains(value, "moe")
	}
}

func (c *HuggingFaceCatalog) fetchTopModels(ctx context.Context, params normalizedParams) ([]listedModel, json.RawMessage, error) {
	endpoint, err := url.Parse(c.baseURL + "/api/models")
	if err != nil {
		return nil, nil, err
	}
	query := endpoint.Query()
	query.Set("pipeline_tag", "text-generation")
	query.Set("sort", hfSort(params.Sort))
	query.Set("direction", "-1")
	query.Set("limit", fmt.Sprintf("%d", params.Limit))
	if params.Search != "" {
		query.Set("search", params.Search)
	}
	endpoint.RawQuery = query.Encode()
	var listed []listedModel
	raw, err := c.fetchJSON(ctx, endpoint.String(), "Hugging Face model list", nil, &listed)
	if err != nil {
		return nil, nil, err
	}
	return listed, raw, nil
}

func (c *HuggingFaceCatalog) fetchModelInfoRaw(ctx context.Context, source Source) (modelInfo, json.RawMessage, error) {
	endpoint := c.baseURL + "/api/models/" + repoEscaped(source) + "?blobs=true"
	var info modelInfo
	raw, err := c.fetchJSON(ctx, endpoint, "Hugging Face metadata", fmt.Errorf("%w: Hugging Face repo not found", ErrNotFound), &info)
	return info, raw, err
}

func (c *HuggingFaceCatalog) fetchDescription(ctx context.Context, source Source) string {
	raw, status, err := c.fetchBytes(ctx, c.baseURL+"/"+repoEscaped(source)+"/raw/main/README.md")
	if err != nil || status < 200 || status >= 300 {
		return ""
	}
	return summarizeREADME(string(raw))
}

func summarizeREADME(readme string) string {
	inFence := false
	for _, line := range strings.Split(stripFrontmatter(readme), "\n") {
		text := strings.TrimSpace(line)
		if strings.HasPrefix(text, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if skipDescriptionLine(text) {
			continue
		}
		return truncateWords(text, 280)
	}
	return ""
}

func stripFrontmatter(readme string) string {
	text := strings.TrimLeft(readme, "\ufeff\r\n\t ")
	if !strings.HasPrefix(text, "---") {
		return readme
	}
	for i, lines := 1, strings.Split(text, "\n"); i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return readme
}

func skipDescriptionLine(line string) bool {
	return line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") || strings.Contains(line, "[![") || strings.HasPrefix(line, "![")
}

func truncateWords(text string, maxLen int) string {
	if len(text) <= maxLen {
		return strings.TrimSpace(text)
	}
	cut := strings.LastIndexAny(text[:maxLen], " \t\n")
	if cut <= 0 {
		cut = maxLen
	}
	return strings.TrimSpace(text[:cut]) + "…"
}

func (c *HuggingFaceCatalog) fetchJSON(ctx context.Context, endpoint string, label string, notFound error, into any) (json.RawMessage, error) {
	raw, status, err := c.fetchBytes(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", label, err)
	}
	if status == http.StatusNotFound && notFound != nil {
		return nil, notFound
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("fetch %s: status %d", label, status)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return raw, nil
}

// fetchBytes performs a GET request and returns the response status and,
// for a 2xx response, its body. err is only set for request/transport
// failures, not for non-2xx statuses.
func (c *HuggingFaceCatalog) fetchBytes(ctx context.Context, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20)) // drain so the connection is reusable
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	return raw, resp.StatusCode, err
}

// repoEscaped returns the URL-safe "owner/repo" path segment for source.
func repoEscaped(source Source) string {
	return url.PathEscape(source.Owner) + "/" + url.PathEscape(source.Repo)
}

func (c *HuggingFaceCatalog) filesFor(source Source, siblings []sibling) ([]File, error) {
	out := make([]File, 0, len(siblings))
	for _, item := range siblings {
		if !strings.HasSuffix(strings.ToLower(item.RFilename), ".gguf") {
			continue
		}
		localPath, err := c.LocalPath(source, item.RFilename)
		if err != nil {
			return nil, err
		}
		size := item.Size
		if item.LFS != nil && item.LFS.Size > 0 {
			size = item.LFS.Size
		}
		_, statErr := os.Stat(localPath)
		out = append(out, File{
			Filename:     item.RFilename,
			Quant:        InferQuant(item.RFilename),
			SizeBytes:    size,
			Downloadable: true,
			LocalPath:    localPath,
			Exists:       statErr == nil,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Filename < out[j].Filename })
	return out, nil
}

func (c *HuggingFaceCatalog) LocalPath(source Source, filename string) (string, error) {
	if err := validateRelativeFilePath(filename); err != nil {
		return "", err
	}
	target := filepath.Join(c.modelStorageDir, source.Owner, source.Repo, filepath.FromSlash(filename))
	cleanRoot := filepath.Clean(c.modelStorageDir)
	cleanTarget := filepath.Clean(target)
	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", fmt.Errorf("%w: model path escapes storage dir", ErrInvalidInput)
	}
	return cleanTarget, nil
}

func (c *HuggingFaceCatalog) DownloadURL(source Source, filename string) (string, error) {
	if err := validateRelativeFilePath(filename); err != nil {
		return "", err
	}
	segments := strings.Split(filename, "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		escaped = append(escaped, url.PathEscape(segment))
	}
	return c.baseURL + "/" + repoEscaped(source) + "/resolve/main/" + strings.Join(escaped, "/"), nil
}

func validateRelativeFilePath(filename string) error {
	if filename == "" || filepath.IsAbs(filename) || strings.Contains(filename, `\`) {
		return fmt.Errorf("%w: invalid model filename", ErrInvalidInput)
	}
	for _, part := range strings.Split(filename, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: invalid model filename", ErrInvalidInput)
		}
	}
	return nil
}

func normalizeListRequest(req ListRequest) normalizedParams {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 100)
	sortValue := req.Sort
	if sortValue != "trending" && sortValue != "modified" {
		sortValue = "downloads"
	}
	minFit := req.MinFit
	if minFit != "marginal" && minFit != "all" {
		minFit = "fits"
	}
	return normalizedParams{Limit: limit, Sort: sortValue, Search: strings.TrimSpace(req.Search), MinFit: minFit, Task: strings.TrimSpace(req.Task), LocalOnly: req.LocalOnly}
}

func hfSort(sortValue string) string {
	switch sortValue {
	case "trending":
		return "trendingScore"
	case "modified":
		return "lastModified"
	default:
		return "downloads"
	}
}

func sourceFromModelID(id string) (Source, bool) {
	parts := strings.Split(strings.Trim(id, "/"), "/")
	if len(parts) != 2 || !safeSegment(parts[0]) || !safeSegment(parts[1]) {
		return Source{}, false
	}
	urlValue := "https://huggingface.co/" + parts[0] + "/" + parts[1]
	return Source{Kind: "huggingface", Owner: parts[0], Repo: parts[1], URL: urlValue}, true
}

func bestFile(files []File) File {
	best := files[0]
	for _, file := range files[1:] {
		if compareFile(file, best) > 0 {
			best = file
		}
	}
	return best
}

func compareFile(a File, b File) int {
	if a.Exists != b.Exists {
		if a.Exists {
			return 1
		}
		return -1
	}
	if fitRank(a.FitLevel) != fitRank(b.FitLevel) {
		return fitRank(a.FitLevel) - fitRank(b.FitLevel)
	}
	if quantRank(a.Quant) != quantRank(b.Quant) {
		return quantRank(a.Quant) - quantRank(b.Quant)
	}
	if a.SizeBytes != b.SizeBytes {
		if a.SizeBytes < b.SizeBytes {
			return 1
		}
		return -1
	}
	return strings.Compare(b.Filename, a.Filename)
}

func quantRank(quant string) int {
	switch {
	case strings.HasPrefix(quant, "Q4_K_M"):
		return 8
	case strings.HasPrefix(quant, "Q5_K_M"):
		return 7
	case strings.HasPrefix(quant, "Q6_K"):
		return 6
	case strings.HasPrefix(quant, "Q4"):
		return 5
	case strings.HasPrefix(quant, "Q5"):
		return 4
	case strings.HasPrefix(quant, "Q8"):
		return 3
	case strings.HasPrefix(quant, "F16"), strings.HasPrefix(quant, "BF16"):
		return 2
	default:
		return 1
	}
}

func scoreModel(model CatalogModel) float64 {
	score := float64(fitRank(model.Fit.Level)) * 1000
	if model.BestFile != nil && model.BestFile.Exists {
		score += 500
	}
	score += float64(quantRank(model.BestFile.Quant)) * 20
	score += math.Log10(float64(max(model.Downloads, 0)) + 1)
	score += math.Log10(float64(max(model.Likes, 0)) + 1)
	if modified, err := time.Parse(time.RFC3339, model.LastModified); err == nil {
		days := time.Since(modified).Hours() / 24
		if days < 365 {
			score += (365 - days) / 365
		}
	}
	return score
}

func mergeTags(left []string, right []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, tag := range append(left, right...) {
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func licenseFromTags(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, "license:") {
			return strings.TrimPrefix(tag, "license:")
		}
	}
	return ""
}

func (c *HuggingFaceCatalog) cacheTTL() time.Duration {
	if c.cache == nil {
		return 0
	}
	return c.cache.ttl
}
