package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"llamarig/config"
	"llamarig/core/configstore"
	"llamarig/core/modelpresets"
	"llamarig/core/router"
	"llamarig/core/runtime"
)

type recordingLocalModels struct {
	deleted []string
	err     error
}

func (f *recordingLocalModels) DeleteLocal(_ context.Context, path string) error {
	f.deleted = append(f.deleted, path)
	return f.err
}

func TestCleanupPresetRemovesConfigReferences(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("router:\n  default_preset: broken\n  autostart_presets: [broken, keep]\n  models_max: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "broken", Values: map[string]string{"model": "/missing.gguf"}}, true); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Dependencies{
		Config:       configstore.NewFileStore(configPath, configstore.DefaultLimitBytes),
		Presets:      store,
		RouterConfig: config.RouterConfig{DefaultPreset: "broken", AutostartPresets: []string{"broken", "keep"}},
	})

	if _, err := manager.CleanupPreset(context.Background(), "broken"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "broken"); !errors.Is(err, modelpresets.ErrNotFound) {
		t.Fatalf("preset remains: %v", err)
	}
	cfg := manager.routerConfigSnapshot(context.Background())
	if cfg.DefaultPreset != "" || len(cfg.AutostartPresets) != 1 || cfg.AutostartPresets[0] != "keep" {
		t.Fatalf("router config = %#v", cfg)
	}
}

func TestCleanupPresetRejectsAvailablePreset(t *testing.T) {
	model := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(model, []byte("gguf"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "ready", Values: map[string]string{"model": model}}, true); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Dependencies{Presets: store})
	if result, err := manager.CleanupPreset(context.Background(), "ready"); Kind(err) != ErrorConflict || result.Status != "" {
		t.Fatalf("CleanupPreset() = %#v, %v", result, err)
	}
}

func TestDeleteLocalModelFailurePreservesPreset(t *testing.T) {
	model := filepath.Join(t.TempDir(), "model.gguf")
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "exact", Values: map[string]string{"model": model}}, true); err != nil {
		t.Fatal(err)
	}
	local := &recordingLocalModels{err: errors.New("permission denied")}
	manager := NewManager(Dependencies{Presets: store, LocalModels: local})
	result, err := manager.DeleteLocalModel(context.Background(), model, true)
	if err == nil || result.Status != "" {
		t.Fatalf("DeleteLocalModel() = %#v, %v", result, err)
	}
	if _, err := store.Get(context.Background(), "exact"); err != nil {
		t.Fatalf("preset removed after failed model deletion: %v", err)
	}
}

func TestDeleteLocalModelCascadesExactPresetOnly(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model.gguf")
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	for _, section := range []modelpresets.Section{
		{Name: "exact", Values: map[string]string{"model": model}},
		{Name: "directory", Values: map[string]string{"models-dir": root}},
	} {
		if err := store.Put(context.Background(), section, true); err != nil {
			t.Fatal(err)
		}
	}
	local := &recordingLocalModels{}
	manager := NewManager(Dependencies{Presets: store, LocalModels: local})
	if _, err := manager.DeleteLocalModel(context.Background(), model, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "exact"); !errors.Is(err, modelpresets.ErrNotFound) {
		t.Fatalf("exact preset remains: %v", err)
	}
	if _, err := store.Get(context.Background(), "directory"); err != nil {
		t.Fatalf("directory preset removed: %v", err)
	}
	if len(local.deleted) != 1 || local.deleted[0] != model {
		t.Fatalf("deleted = %v", local.deleted)
	}
}

func TestDeleteLocalModelRequiresCascadeAndRejectsActivePreset(t *testing.T) {
	model := filepath.Join(t.TempDir(), "model.gguf")
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "exact", Values: map[string]string{"model": model}}, true); err != nil {
		t.Fatal(err)
	}
	local := &recordingLocalModels{}
	manager := NewManager(Dependencies{Presets: store, LocalModels: local})
	if _, err := manager.DeleteLocalModel(context.Background(), model, false); Kind(err) != ErrorConflict {
		t.Fatalf("DeleteLocalModel() without cascade error = %v", err)
	}

	loaded := router.Model{ID: "exact"}
	loaded.Status.Value = "loaded"
	manager = NewManager(Dependencies{
		Presets:       store,
		LocalModels:   local,
		Router:        &fakeRouterClient{models: []router.Model{loaded}},
		RouterRuntime: &fakeRuntime{name: "router", state: runtime.Running},
	})
	if _, err := manager.DeleteLocalModel(context.Background(), model, true); Kind(err) != ErrorConflict {
		t.Fatalf("DeleteLocalModel() active error = %v", err)
	}
	if len(local.deleted) != 0 {
		t.Fatalf("deleted = %v", local.deleted)
	}
}

func TestDeleteLocalModelRejectsActiveDirectoryPreset(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model.gguf")
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "directory", Values: map[string]string{"models-dir": root}}, true); err != nil {
		t.Fatal(err)
	}
	loaded := router.Model{ID: "directory"}
	loaded.Status.Value = "loaded"
	local := &recordingLocalModels{}
	manager := NewManager(Dependencies{Presets: store, LocalModels: local, Router: &fakeRouterClient{models: []router.Model{loaded}}, RouterRuntime: &fakeRuntime{name: "router", state: runtime.Running}})
	if _, err := manager.DeleteLocalModel(context.Background(), model, true); Kind(err) != ErrorConflict {
		t.Fatalf("DeleteLocalModel() error = %v", err)
	}
	if len(local.deleted) != 0 {
		t.Fatalf("deleted = %v", local.deleted)
	}
}

func TestValidateModelCascadeReadsRouterOnce(t *testing.T) {
	routerClient := &fakeRouterClient{}
	routerRuntime := &fakeRuntime{name: "router", state: runtime.Running}
	manager := NewManager(Dependencies{Router: routerClient, RouterRuntime: routerRuntime})
	if err := manager.validateModelCascade(context.Background(), []string{"one", "two"}, []string{"directory"}, true); err != nil {
		t.Fatal(err)
	}
	if routerRuntime.statusCalls != 1 || routerClient.lists != 1 {
		t.Fatalf("status calls=%d list calls=%d", routerRuntime.statusCalls, routerClient.lists)
	}
}
