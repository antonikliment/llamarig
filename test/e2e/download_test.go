package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestDownloadLifecycle(t *testing.T) {
	client := startService(t)
	ctx := context.Background()

	started, err := client.StartModelDownload(ctx, &controlv1.StartModelDownloadRequest{
		Url:      "https://huggingface.co/ggml-org/models",
		Filename: "tinyllamas/stories260K.gguf",
		Force:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "StartModelDownload", started.GetOk())

	download := waitDownload(t, client.GetModelDownload, started.GetDownload().GetId())
	if download.GetState() != "completed" {
		t.Fatalf("download state=%q error=%q", download.GetState(), download.GetError())
	}
	if download.GetTargetPath() == "" {
		t.Fatalf("download path empty")
	}
	if _, err := os.Stat(download.GetTargetPath()); err != nil {
		t.Fatalf("stat download: %v", err)
	}
	if download.GetTotalBytes() != 0 && download.GetReceivedBytes() != download.GetTotalBytes() {
		t.Fatalf("downloaded=%d total=%d", download.GetReceivedBytes(), download.GetTotalBytes())
	}
}

func waitDownload(t *testing.T, get func(context.Context, *controlv1.GetModelDownloadRequest) (*controlv1.ModelDownloadResponse, error), id string) *controlv1.ModelDownload {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
			resp, err := get(ctx, &controlv1.GetModelDownloadRequest{Id: id})
			if err != nil {
				t.Fatal(err)
			}
			download := resp.GetDownload()
			switch download.GetState() {
			case "completed", "failed":
				return download
			}
		}
	}
}
