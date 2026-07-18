package modeldownload

import (
	"context"
	"errors"
)

var (
	ErrInvalidInput = errors.New("model download input is invalid")
	ErrNotFound     = errors.New("model download not found")
	ErrConflict     = errors.New("model download conflict")
)

const (
	StateQueued            = "queued"
	StateRunning           = "running"
	StateCompleted         = "completed"
	StateFailed            = "failed"
	StateCancelled         = "cancelled"
	StateAlreadyDownloaded = "already_downloaded"
)

type Request struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Force    bool   `json:"force,omitempty"`
}

type Job struct {
	ID            string  `json:"id"`
	State         string  `json:"state"`
	URL           string  `json:"url"`
	Filename      string  `json:"filename"`
	TargetPath    string  `json:"target_path"`
	ReceivedBytes int64   `json:"received_bytes"`
	TotalBytes    int64   `json:"total_bytes"`
	Percent       float64 `json:"percent"`
	Error         string  `json:"error,omitempty"`
	StartedAt     string  `json:"started_at,omitempty"`
	CompletedAt   string  `json:"completed_at,omitempty"`
}

type Downloader interface {
	Start(ctx context.Context, req Request) (Job, error)
	Get(ctx context.Context, id string) (Job, error)
	Cancel(ctx context.Context, id string) (Job, error)
}
