package modeldownload

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"llamarig/core/modelcatalog"
	"llamarig/platform/filedoc"
)

type Catalog interface {
	Resolve(ctx context.Context, rawURL string) (modelcatalog.Resolution, error)
}

type DownloadURLer interface {
	DownloadURL(source modelcatalog.Source, filename string) (string, error)
}

type Manager struct {
	catalog     Catalog
	downloadURL DownloadURLer
	client      *http.Client
	mu          sync.Mutex
	jobs        map[string]*Job
	active      map[string]string
	cancel      map[string]context.CancelFunc
}

type Dependencies struct {
	Catalog     Catalog
	DownloadURL DownloadURLer
	HTTPClient  *http.Client
}

func NewManager(deps Dependencies) *Manager {
	client := deps.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Manager{catalog: deps.Catalog, downloadURL: deps.DownloadURL, client: client, jobs: map[string]*Job{}, active: map[string]string{}, cancel: map[string]context.CancelFunc{}}
}

func (m *Manager) Start(ctx context.Context, req Request) (Job, error) {
	if req.URL == "" || req.Filename == "" {
		return Job{}, fmt.Errorf("%w: url and filename are required", ErrInvalidInput)
	}
	if m.catalog == nil || m.downloadURL == nil {
		return Job{}, fmt.Errorf("%w: downloader is not configured", ErrInvalidInput)
	}
	resolution, err := m.catalog.Resolve(ctx, req.URL)
	if err != nil {
		return Job{}, err
	}
	sourceFile, ok := findFile(resolution.Files, req.Filename)
	if !ok {
		return Job{}, fmt.Errorf("%w: selected GGUF file not found in resolved repo", ErrInvalidInput)
	}
	if sourceFile.Exists && !req.Force {
		job := alreadyDownloadedJob(req, sourceFile)
		stored := job
		m.mu.Lock()
		m.jobs[job.ID] = &stored
		m.mu.Unlock()
		return job, nil
	}
	downloadURL, err := m.downloadURL.DownloadURL(resolution.Source, req.Filename)
	if err != nil {
		return Job{}, err
	}
	m.mu.Lock()
	if existingID := m.active[sourceFile.LocalPath]; existingID != "" {
		job := *m.jobs[existingID]
		m.mu.Unlock()
		return job, nil
	}
	job := queuedJob(req, sourceFile)
	stored := job
	m.jobs[job.ID] = &stored
	m.active[job.TargetPath] = job.ID
	downloadCtx, cancel := context.WithCancel(context.Background())
	m.cancel[job.ID] = cancel
	m.mu.Unlock()

	go m.run(downloadCtx, job.ID, downloadURL)
	return job, nil
}

func (m *Manager) Get(ctx context.Context, id string) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return *state, nil
}

func (m *Manager) Cancel(ctx context.Context, id string) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil {
		return Job{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if job.State != StateQueued && job.State != StateRunning {
		return Job{}, fmt.Errorf("%w: download %s is %s", ErrConflict, id, job.State)
	}
	if cancel := m.cancel[id]; cancel != nil {
		cancel()
	}
	job.State = StateCancelled
	job.CompletedAt = timestamp()
	return *job, nil
}

func (m *Manager) run(ctx context.Context, id string, downloadURL string) {
	m.update(id, func(job *Job) {
		if job.State == StateCancelled {
			return
		}
		job.State = StateRunning
		job.StartedAt = timestamp()
	})
	defer m.clearActive(id)
	err := m.download(ctx, id, downloadURL)
	m.update(id, func(job *Job) {
		job.CompletedAt = timestamp()
		if job.State == StateCancelled || errors.Is(err, context.Canceled) {
			job.State, job.Error = StateCancelled, ""
			return
		}
		if err != nil {
			job.State, job.Error = StateFailed, err.Error()
			return
		}
		job.State, job.Percent = StateCompleted, 100
	})
}

func (m *Manager) download(ctx context.Context, id string, downloadURL string) error {
	job, err := m.Get(context.Background(), id)
	if err != nil {
		return err
	}
	partPath := job.TargetPath + ".part"
	if err := prepareTarget(job.TargetPath, partPath); err != nil {
		return err
	}
	resp, err := m.openDownload(ctx, downloadURL, partPath)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	total := resp.ContentLength
	m.update(id, func(job *Job) {
		if total > 0 {
			job.TotalBytes = total
		}
	})
	out, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create partial model file: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = out.Close()
		}
	}()
	buf := make([]byte, 1024*1024)
	err = m.copyDownload(id, out, resp.Body, buf)
	if err != nil {
		_ = os.Remove(partPath)
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(partPath)
		return err
	}
	if err := out.Sync(); err != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("sync partial model file: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("close partial model file: %w", err)
	}
	closed = true
	return finalizeDownload(partPath, job.TargetPath)
}

func (m *Manager) openDownload(ctx context.Context, downloadURL string, partPath string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download model: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	_ = resp.Body.Close()
	_ = os.Remove(partPath)
	return nil, fmt.Errorf("download model: status %d", resp.StatusCode)
}

func (m *Manager) copyDownload(id string, out *os.File, body io.Reader, buf []byte) error {
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return fmt.Errorf("write partial model file: %w", err)
			}
			m.addProgress(id, int64(n))
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read model download: %w", readErr)
		}
	}
}

func finalizeDownload(partPath string, targetPath string) error {
	if err := rejectSymlink(targetPath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(partPath)
		return err
	}
	if err := os.Rename(partPath, targetPath); err != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("finalize model file: %w", err)
	}
	return filedoc.SyncDir(filepath.Dir(targetPath))
}

func (m *Manager) update(id string, fn func(*Job)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state := m.jobs[id]; state != nil {
		fn(state)
	}
}

func (m *Manager) addProgress(id string, n int64) {
	m.update(id, func(job *Job) {
		job.ReceivedBytes += n
		if job.TotalBytes > 0 {
			job.Percent = float64(job.ReceivedBytes) * 100 / float64(job.TotalBytes)
		}
	})
}

func (m *Manager) clearActive(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.jobs[id]
	if state == nil {
		return
	}
	if m.active[state.TargetPath] == id {
		delete(m.active, state.TargetPath)
	}
	delete(m.cancel, id)
}

func findFile(files []modelcatalog.File, filename string) (modelcatalog.File, bool) {
	for _, file := range files {
		if file.Filename == filename {
			return file, true
		}
	}
	return modelcatalog.File{}, false
}

func alreadyDownloadedJob(req Request, file modelcatalog.File) Job {
	return Job{ID: newID(), State: StateAlreadyDownloaded, URL: req.URL, Filename: req.Filename, TargetPath: file.LocalPath, ReceivedBytes: file.SizeBytes, TotalBytes: file.SizeBytes, Percent: 100, CompletedAt: timestamp()}
}

func queuedJob(req Request, file modelcatalog.File) Job {
	return Job{ID: newID(), State: StateQueued, URL: req.URL, Filename: req.Filename, TargetPath: file.LocalPath, TotalBytes: file.SizeBytes}
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func newID() string {
	var random [3]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("dl_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("dl_%s_%s", time.Now().UTC().Format("20060102_150405"), hex.EncodeToString(random[:]))
}
