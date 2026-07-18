package rpc

import (
	"path/filepath"
	"testing"

	"llamarig/core/modelcatalog"
	"llamarig/core/modelpresets"
)

func TestPresetReferences(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "owner", "model.gguf")
	presets := []modelpresets.Section{
		{Name: "exact", Values: map[string]string{"model": model}},
		{Name: "directory", Values: map[string]string{"models-dir": root}},
		{Name: "other", Values: map[string]string{"model": filepath.Join(root, "other.gguf")}},
	}
	refs := modelpresets.FindReferences(presets, model)
	if len(refs.ModelPaths) != 1 || refs.ModelPaths[0] != "exact" || len(refs.ModelDirs) != 1 || refs.ModelDirs[0] != "directory" {
		t.Fatalf("refs = %#v", refs)
	}
}

func TestPathContainsRejectsSibling(t *testing.T) {
	root := t.TempDir()
	if modelcatalog.PathContains(filepath.Join(root, "models"), filepath.Join(root, "models-old", "x.gguf")) {
		t.Fatal("sibling path treated as contained")
	}
}
