package modelcatalog

import (
	"context"
	"errors"
)

var (
	ErrInvalidInput = errors.New("model catalog input is invalid")
	ErrNotFound     = errors.New("model catalog entry not found")
)

type Source struct {
	Kind  string `json:"kind"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	URL   string `json:"url"`
}

type LlamaCPPCompatibility struct {
	Compatible bool   `json:"compatible"`
	HFRef      string `json:"hf_ref,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type File struct {
	Filename           string `json:"filename"`
	Quant              string `json:"quant,omitempty"`
	SizeBytes          int64  `json:"size_bytes,omitempty"`
	Downloadable       bool   `json:"downloadable"`
	LocalPath          string `json:"local_path"`
	Exists             bool   `json:"exists"`
	EstimatedRAMBytes  int64  `json:"estimated_ram_bytes,omitempty"`
	EstimatedVRAMBytes int64  `json:"estimated_vram_bytes,omitempty"`
	FitLevel           string `json:"fit_level,omitempty"`
	FitReason          string `json:"fit_reason,omitempty"`
}

type Resolution struct {
	OK            bool                  `json:"ok"`
	Source        Source                `json:"source"`
	LlamaCPP      LlamaCPPCompatibility `json:"llama_cpp"`
	Description   string                `json:"description,omitempty"`
	Params        int64                 `json:"params,omitempty"`
	Architecture  string                `json:"architecture,omitempty"`
	ContextLength int64                 `json:"context_length,omitempty"`
	IsMoE         bool                  `json:"is_moe,omitempty"`
	Files         []File                `json:"files"`
}

type Catalog interface {
	Resolve(ctx context.Context, rawURL string) (Resolution, error)
}

type ListRequest struct {
	Limit     int
	Sort      string
	Search    string
	MinFit    string
	Task      string
	LocalOnly bool
}

type MachineProfile struct {
	TotalRAMBytes     int64  `json:"total_ram_bytes,omitempty"`
	AvailableRAMBytes int64  `json:"available_ram_bytes,omitempty"`
	GPUName           string `json:"gpu_name,omitempty"`
	VRAMBytes         int64  `json:"vram_bytes,omitempty"`
	HasGPU            bool   `json:"has_gpu"`
}

type MachineFit struct {
	Level              string `json:"level"`
	Reason             string `json:"reason,omitempty"`
	RequiredRAMBytes   int64  `json:"required_ram_bytes,omitempty"`
	AvailableRAMBytes  int64  `json:"available_ram_bytes,omitempty"`
	RequiredVRAMBytes  int64  `json:"required_vram_bytes,omitempty"`
	AvailableVRAMBytes int64  `json:"available_vram_bytes,omitempty"`
}

type CatalogModel struct {
	ID            string     `json:"id"`
	Owner         string     `json:"owner"`
	Repo          string     `json:"repo"`
	URL           string     `json:"url"`
	Downloads     int64      `json:"downloads,omitempty"`
	Likes         int64      `json:"likes,omitempty"`
	LastModified  string     `json:"last_modified,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	License       string     `json:"license,omitempty"`
	Params        int64      `json:"params,omitempty"`
	Architecture  string     `json:"architecture,omitempty"`
	ContextLength int64      `json:"context_length,omitempty"`
	IsMoE         bool       `json:"is_moe,omitempty"`
	Files         []File     `json:"files"`
	BestFile      *File      `json:"best_file,omitempty"`
	Fit           MachineFit `json:"fit"`
	Score         float64    `json:"score"`
}

type CacheState struct {
	Hit        bool   `json:"hit"`
	Stale      bool   `json:"stale"`
	Refreshing bool   `json:"refreshing"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	TTLSeconds int64  `json:"ttl_seconds,omitempty"`
}

type ListResult struct {
	OK      bool             `json:"ok"`
	Machine MachineProfile   `json:"machine"`
	Models  []CatalogModel   `json:"models"`
	Cache   CacheState       `json:"cache"`
	Errors  []string         `json:"errors,omitempty"`
	Params  normalizedParams `json:"-"`
}

type normalizedParams struct {
	Limit     int    `json:"limit"`
	Sort      string `json:"sort"`
	Search    string `json:"search,omitempty"`
	MinFit    string `json:"min_fit"`
	Task      string `json:"task,omitempty"`
	LocalOnly bool   `json:"local_only,omitempty"`
}

type Discoverer interface {
	List(ctx context.Context, req ListRequest, machine MachineProfile) (ListResult, error)
}
