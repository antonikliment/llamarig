package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LocalModel struct {
	Path       string
	Filename   string
	SizeBytes  int64
	ModifiedAt time.Time
}

type LocalLister interface {
	ListLocal(context.Context) ([]LocalModel, error)
	DeleteLocal(context.Context, string) error
}

type localWatchState struct {
	current     string
	candidate   string
	initialized bool
}

func (c *HuggingFaceCatalog) ListLocal(ctx context.Context) ([]LocalModel, error) {
	models := make([]LocalModel, 0)
	err := filepath.WalkDir(c.modelStorageDir, func(path string, entry fs.DirEntry, walkErr error) error {
		return collectLocalModel(ctx, c.modelStorageDir, path, entry, walkErr, &models)
	})
	sort.Slice(models, func(i, j int) bool { return models[i].Path < models[j].Path })
	return models, err
}

func (c *HuggingFaceCatalog) DeleteLocal(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.modelStorageDir == "" {
		return fmt.Errorf("%w: model storage directory is not configured", ErrInvalidInput)
	}
	path = CanonicalPath(path)
	root := CanonicalPath(c.modelStorageDir)
	if !PathContains(root, path) {
		return fmt.Errorf("%w: model path escapes storage dir", ErrInvalidInput)
	}
	if !strings.EqualFold(filepath.Ext(path), ".gguf") {
		return fmt.Errorf("%w: local model must be a .gguf file", ErrInvalidInput)
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: local model not found", ErrNotFound)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%w: local model path is a directory", ErrInvalidInput)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	pruneEmptyParents(filepath.Dir(path), root)
	return nil
}

// WatchLocal reports stable changes to the recursive GGUF inventory. Partial
// downloads are absent from ListLocal, and two matching scans debounce writes.
func (c *HuggingFaceCatalog) WatchLocal(ctx context.Context, interval time.Duration) <-chan struct{} {
	changes := make(chan struct{}, 1)
	go c.watchLocal(ctx, interval, changes)
	return changes
}

func (c *HuggingFaceCatalog) watchLocal(ctx context.Context, interval time.Duration, changes chan struct{}) {
	defer close(changes)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	state := localWatchState{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			models, err := c.ListLocal(ctx)
			if err != nil {
				continue
			}
			if state.observe(fmt.Sprint(models)) {
				select {
				case changes <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (s *localWatchState) observe(next string) bool {
	switch {
	case !s.initialized:
		s.current, s.initialized = next, true
	case next == s.current:
		s.candidate = ""
	case next != s.candidate:
		s.candidate = next
	default:
		s.current, s.candidate = next, ""
		return true
	}
	return false
}

func collectLocalModel(ctx context.Context, root string, path string, entry fs.DirEntry, walkErr error, models *[]LocalModel) error {
	if walkErr != nil {
		if path == root && errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		return walkErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if entry.Type()&fs.ModeSymlink != 0 {
		return nil
	}
	if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".gguf") {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		*models = append(*models, LocalModel{Path: filepath.Clean(path), Filename: entry.Name(), SizeBytes: info.Size(), ModifiedAt: info.ModTime().UTC()})
	}
	return nil
}

// CanonicalPath resolves a path to an absolute, symlink-free, cleaned form for
// safe containment checks against the storage root.
func CanonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

// PathContains reports whether path lies within dir (both should be canonical).
func PathContains(dir string, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// pruneEmptyParents removes now-empty parent dirs up to (not including) root.
// Best-effort: any error (non-empty dir, already gone) just stops the walk.
func pruneEmptyParents(dir string, root string) {
	for PathContains(root, dir) && dir != root && os.Remove(dir) == nil {
		dir = filepath.Dir(dir)
	}
}
