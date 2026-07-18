package modelcatalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestHuggingFaceCatalogResolve(t *testing.T) {
	root := t.TempDir()
	server := newResolveTestServer(t)
	defer server.Close()

	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: root, BaseURL: server.URL, HTTPClient: server.Client()})
	resolution, err := catalog.Resolve(context.Background(), "https://huggingface.co/owner/repo")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if !resolution.OK || !resolution.LlamaCPP.Compatible || resolution.LlamaCPP.HFRef != "owner/repo" {
		t.Fatalf("resolution = %#v", resolution)
	}
	if resolution.Description != "Model summary." {
		t.Fatalf("description = %q", resolution.Description)
	}
	if len(resolution.Files) != 2 {
		t.Fatalf("files = %#v", resolution.Files)
	}
	if resolution.Files[0].Filename != "model-Q4_K_M.gguf" || resolution.Files[0].SizeBytes != 456 {
		t.Fatalf("first file = %#v", resolution.Files[0])
	}
	want := filepath.Join(root, "owner", "repo", "nested", "model-Q8_0.gguf")
	if resolution.Files[1].LocalPath != want {
		t.Fatalf("nested local path = %q, want %q", resolution.Files[1].LocalPath, want)
	}
}

func newResolveTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/owner/repo/raw/main/README.md" {
			_, _ = w.Write([]byte("Model summary."))
			return
		}
		if r.URL.Path != "/api/models/owner/repo" || r.URL.Query().Get("blobs") != "true" {
			t.Fatalf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"siblings":[
			{"rfilename":"README.md","size":10},
			{"rfilename":"model-Q4_K_M.gguf","size":123,"lfs":{"size":456}},
			{"rfilename":"nested/model-Q8_0.gguf","size":789}
		]}`))
	}))
}

func TestModelMetadata(t *testing.T) {
	tests := []struct {
		name   string
		info   modelInfo
		params int64
		arch   string
		ctx    int64
		moe    bool
	}{
		{
			name:   "gguf fields win",
			info:   withSafetensors(modelInfo{GGUF: modelGGUF{Total: 7_615_616_512, Architecture: "llama", ContextLength: 32768}}, 1),
			params: 7_615_616_512,
			arch:   "llama",
			ctx:    32768,
		},
		{
			name:   "safetensors config fallback",
			info:   withSafetensors(withConfig(modelInfo{}, []string{"Qwen2ForCausalLM"}, "qwen2", 0), 12),
			params: 12,
			arch:   "Qwen2ForCausalLM",
		},
		{
			name: "num experts marks moe",
			info: withConfig(modelInfo{}, nil, "llama", 2),
			arch: "llama",
			moe:  true,
		},
		{
			name: "known moe arch marks moe",
			info: withConfig(modelInfo{}, nil, "mixtral", 0),
			arch: "mixtral",
			moe:  true,
		},
		{
			name: "moe substring marks moe",
			info: withConfig(modelInfo{}, nil, "custom_moe", 0),
			arch: "custom_moe",
			moe:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, arch, ctxLen, moe := modelMetadata(tt.info)
			if params != tt.params || arch != tt.arch || ctxLen != tt.ctx || moe != tt.moe {
				t.Fatalf("modelMetadata = (%d, %q, %d, %t)", params, arch, ctxLen, moe)
			}
		})
	}
}

func withSafetensors(info modelInfo, total int64) modelInfo {
	info.Safetensors.Total = total
	return info
}

func withConfig(info modelInfo, arch []string, modelType string, experts int64) modelInfo {
	info.Config.Architectures = arch
	info.Config.ModelType = modelType
	info.Config.NumExpertsPerTok = experts
	return info
}

func TestSummarizeREADME(t *testing.T) {
	long := strings.Repeat("word ", 80)
	summary := summarizeREADME("---\nlicense: apache-2.0\n---\n# Title\n[![badge](x)](y)\n| a | b |\n```text\nskip\n```\n" + long)
	if strings.Contains(summary, "license") || strings.Contains(summary, "#") || !strings.HasSuffix(summary, "…") || utf8.RuneCountInString(summary) > 281 {
		t.Fatalf("summary = %q", summary)
	}
	if strings.HasSuffix(summary, " …") {
		t.Fatalf("summary has pre-ellipsis space: %q", summary)
	}
}

func TestHuggingFaceCatalogListUsesCacheAndRefreshesStale(t *testing.T) {
	root := t.TempDir()
	cacheDir := t.TempDir()
	requests := make(chan string, 10)
	server := newCachedCatalogServer(t, requests)
	defer server.Close()

	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: root, CacheDir: cacheDir, CacheTTL: time.Nanosecond, BaseURL: server.URL, HTTPClient: server.Client()})
	machine := MachineProfile{AvailableRAMBytes: 4 * 1024 * 1024 * 1024}
	result, err := catalog.List(context.Background(), ListRequest{Limit: 10, MinFit: "fits"}, machine)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Models) != 1 || result.Models[0].BestFile == nil || result.Models[0].BestFile.FitLevel != "fits" {
		t.Fatalf("result = %#v", result)
	}
	<-requests
	<-requests

	events, unsubscribe := catalog.Subscribe()
	defer unsubscribe()
	time.Sleep(time.Millisecond)
	result, err = catalog.List(context.Background(), ListRequest{Limit: 10, MinFit: "fits"}, machine)
	if err != nil {
		t.Fatalf("cached List returned error: %v", err)
	}
	if !result.Cache.Hit || !result.Cache.Stale || !result.Cache.Refreshing {
		t.Fatalf("cache = %#v", result.Cache)
	}
	waitForCatalogRequest(t, requests, "expected background refresh request")
	waitForRefreshEvent(t, events)
}

func waitForCatalogRequest(t *testing.T, requests <-chan string, message string) {
	t.Helper()
	select {
	case <-requests:
	case <-time.After(time.Second):
		t.Fatal(message)
	}
}

func waitForRefreshEvent(t *testing.T, events <-chan RefreshEvent) {
	t.Helper()
	select {
	case event := <-events:
		if !event.OK {
			t.Fatalf("refresh event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected background refresh completion")
	}
}

func newCachedCatalogServer(t *testing.T, requests chan<- string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.String()
		switch r.URL.Path {
		case "/api/models":
			_, _ = w.Write([]byte(`[{"id":"owner/repo","downloads":100,"likes":7,"lastModified":"2026-01-01T00:00:00Z","tags":["license:apache-2.0"]}]`))
		case "/api/models/owner/repo":
			_, _ = w.Write([]byte(`{"downloads":100,"likes":7,"lastModified":"2026-01-01T00:00:00Z","tags":["license:apache-2.0"],"siblings":[{"rfilename":"model-Q4_K_M.gguf","size":1048576}]}`))
		default:
			t.Fatalf("unexpected request %s", r.URL.String())
		}
	}))
}

func TestHuggingFaceCatalogListMarksLocalFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "owner", "repo", "model-Q4_K_M.gguf")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("mkdir model dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("model"), 0o600); err != nil {
		t.Fatalf("write model: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models":
			_, _ = w.Write([]byte(`[{"id":"owner/repo"}]`))
		case "/api/models/owner/repo":
			_, _ = w.Write([]byte(`{"siblings":[{"rfilename":"model-Q4_K_M.gguf","size":1048576}]}`))
		default:
			t.Fatalf("unexpected request %s", r.URL.String())
		}
	}))
	defer server.Close()

	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: root, BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := catalog.List(context.Background(), ListRequest{Limit: 10, MinFit: "all"}, MachineProfile{AvailableRAMBytes: 4 * 1024 * 1024 * 1024})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Models) != 1 || result.Models[0].BestFile == nil || !result.Models[0].BestFile.Exists {
		t.Fatalf("result = %#v", result)
	}
}

func TestHuggingFaceCatalogListReturnsPartialResultsWithErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models":
			_, _ = w.Write([]byte(`[{"id":"owner/good"},{"id":"owner/broken"}]`))
		case "/api/models/owner/good":
			_, _ = w.Write([]byte(`{"siblings":[{"rfilename":"model-Q4_K_M.gguf","size":1048576}]}`))
		case "/api/models/owner/broken":
			http.Error(w, "temporary failure", http.StatusBadGateway)
		default:
			t.Fatalf("unexpected request %s", r.URL.String())
		}
	}))
	defer server.Close()

	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := catalog.List(context.Background(), ListRequest{Limit: 10, MinFit: "all"}, MachineProfile{AvailableRAMBytes: 4 * 1024 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 1 || result.Models[0].ID != "owner/good" || len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "owner/broken") {
		t.Fatalf("result = %#v", result)
	}
}

func TestHuggingFaceCatalogFallsBackFromCorruptCache(t *testing.T) {
	cacheDir := t.TempDir()
	server := newCachedCatalogServer(t, make(chan string, 4))
	defer server.Close()
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: t.TempDir(), CacheDir: cacheDir, CacheTTL: time.Hour, BaseURL: server.URL, HTTPClient: server.Client()})
	req := ListRequest{Limit: 10, MinFit: "all"}
	cachePath := catalog.cache.path(normalizeListRequest(req))
	if err := os.WriteFile(cachePath, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := catalog.List(context.Background(), req, MachineProfile{AvailableRAMBytes: 4 * 1024 * 1024 * 1024})
	if err != nil || len(result.Models) != 1 {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	data, err := os.ReadFile(cachePath)
	if err != nil || !strings.Contains(string(data), `"updated_at"`) {
		t.Fatalf("cache was not replaced: %s error=%v", data, err)
	}
}
