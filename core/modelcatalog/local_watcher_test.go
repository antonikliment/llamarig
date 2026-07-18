package modelcatalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchLocalIgnoresPartialDownloadUntilFinalRename(t *testing.T) {
	dir := t.TempDir()
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: dir})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := catalog.WatchLocal(ctx, 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	partial := filepath.Join(dir, "model.gguf.part")
	if err := os.WriteFile(partial, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
		t.Fatal("partial download produced change")
	case <-time.After(35 * time.Millisecond):
	}
	if err := os.Rename(partial, filepath.Join(dir, "model.gguf")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for finalized model")
	}
}
