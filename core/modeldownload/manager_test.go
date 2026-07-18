package modeldownload

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"llamarig/core/modelcatalog"
)

type fakeCatalog struct {
	resolution modelcatalog.Resolution
}

func (f *fakeCatalog) Resolve(context.Context, string) (modelcatalog.Resolution, error) {
	return f.resolution, nil
}

type fakeURLer struct {
	url string
}

func (f *fakeURLer) DownloadURL(modelcatalog.Source, string) (string, error) {
	return f.url, nil
}

func TestManagerDownloadCompletes(t *testing.T) {
	body := "model data"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	target := filepath.Join(t.TempDir(), "owner", "repo", "model.gguf")
	manager := NewManager(Dependencies{
		Catalog: &fakeCatalog{resolution: resolutionFor(target, int64(len(body)))},
		DownloadURL: &fakeURLer{
			url: server.URL,
		},
		HTTPClient: server.Client(),
	})
	job, err := manager.Start(context.Background(), Request{URL: "https://huggingface.co/owner/repo", Filename: "model.gguf"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	job = waitJob(t, manager, job.ID)
	if job.State != StateCompleted || job.Percent != 100 {
		t.Fatalf("job = %#v", job)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != body {
		t.Fatalf("target data = %q", string(data))
	}
	if _, err := os.Stat(target + ".part"); !os.IsNotExist(err) {
		t.Fatalf("partial file exists or stat failed: %v", err)
	}
}

func TestManagerAlreadyDownloaded(t *testing.T) {
	target := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(target, []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Dependencies{
		Catalog:     &fakeCatalog{resolution: resolutionFor(target, 4)},
		DownloadURL: &fakeURLer{url: "http://127.0.0.1/model.gguf"},
	})
	job, err := manager.Start(context.Background(), Request{URL: "https://huggingface.co/owner/repo", Filename: "model.gguf"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if job.State != StateAlreadyDownloaded || job.Percent != 100 {
		t.Fatalf("job = %#v", job)
	}
}

func TestManagerDuplicateActiveDownloadReturnsExistingJob(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-block
		_, _ = w.Write([]byte("model data"))
	}))
	defer server.Close()
	defer close(block)
	target := filepath.Join(t.TempDir(), "model.gguf")
	manager := NewManager(Dependencies{
		Catalog:     &fakeCatalog{resolution: resolutionFor(target, 10)},
		DownloadURL: &fakeURLer{url: server.URL},
		HTTPClient:  server.Client(),
	})
	first, err := manager.Start(context.Background(), Request{URL: "https://huggingface.co/owner/repo", Filename: "model.gguf"})
	if err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}
	second, err := manager.Start(context.Background(), Request{URL: "https://huggingface.co/owner/repo", Filename: "model.gguf"})
	if err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate id = %q, want %q", second.ID, first.ID)
	}
}

func TestManagerCancelStopsDownload(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	target := filepath.Join(t.TempDir(), "model.gguf")
	manager := NewManager(Dependencies{
		Catalog:     &fakeCatalog{resolution: resolutionFor(target, 10)},
		DownloadURL: &fakeURLer{url: server.URL},
		HTTPClient:  server.Client(),
	})
	job, err := manager.Start(context.Background(), Request{URL: "https://huggingface.co/owner/repo", Filename: "model.gguf"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-started
	job, err = manager.Cancel(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if job.State != StateCancelled {
		t.Fatalf("job state = %q, want %q", job.State, StateCancelled)
	}
	job = waitJob(t, manager, job.ID)
	if job.State != StateCancelled {
		t.Fatalf("final job state = %q, want %q", job.State, StateCancelled)
	}
}

func resolutionFor(target string, size int64) modelcatalog.Resolution {
	return modelcatalog.Resolution{
		OK: true,
		Source: modelcatalog.Source{
			Kind:  "huggingface",
			Owner: "owner",
			Repo:  "repo",
			URL:   "https://huggingface.co/owner/repo",
		},
		Files: []modelcatalog.File{{
			Filename:     "model.gguf",
			SizeBytes:    size,
			Downloadable: true,
			LocalPath:    target,
			Exists:       fileExists(target),
		}},
	}
}

func waitJob(t *testing.T, manager *Manager, id string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := manager.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if job.State == StateCompleted || job.State == StateFailed || job.State == StateCancelled {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("download did not finish")
	return Job{}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
