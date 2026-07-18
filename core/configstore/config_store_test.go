package configstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llamarig/config"
)

func TestFileStoreReadAndReplace(t *testing.T) {
	path := writeConfig(t, `listen_addr: "127.0.0.1:7000"
router:
  default_preset: "default"
`)
	store := NewFileStore(path, 1024*1024)
	ctx := context.Background()

	cfg, err := store.Read(ctx)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if cfg.Content == "" || cfg.SHA256 == "" || cfg.Parsed.Router.DefaultPreset != "default" {
		t.Fatalf("cfg = %#v", cfg)
	}

	result, err := store.Replace(ctx, `listen_addr: "127.0.0.1:7100"
router:
  default_preset: "qwen"
`)
	if err != nil {
		t.Fatalf("Replace returned error: %v", err)
	}
	if result.BackupPath == "" || result.SHA256 == "" {
		t.Fatalf("result = %#v", result)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backup), `default`) {
		t.Fatalf("backup = %s", backup)
	}
}

func TestFileStoreRejectsInvalidContent(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "config.yaml"), 8)
	if err := store.Validate(context.Background(), " \n"); err == nil {
		t.Fatal("expected empty error")
	}
	if err := store.Validate(context.Background(), strings.Repeat("x", 16)); err == nil {
		t.Fatal("expected too large error")
	}
	if err := store.Validate(context.Background(), "runtime:\n  servers: []\n"); err == nil {
		t.Fatal("expected malformed legacy field error")
	}
}

func TestRemoveRouterPresetReferencesPreservesUnrelatedConfig(t *testing.T) {
	path := writeConfig(t, "# keep this comment\nlisten_addr: 127.0.0.1:7000\nrouter:\n  default_preset: broken\n  autostart_presets: [broken, keep]\n  models_max: 2\n")
	store := NewFileStore(path, DefaultLimitBytes)
	if err := store.RemoveRouterPresetReferences(context.Background(), "broken"); err != nil {
		t.Fatal(err)
	}
	document, err := store.Read(context.Background())
	if err != nil || document.Parsed.Router.DefaultPreset != "" || len(document.Parsed.Router.AutostartPresets) != 1 || document.Parsed.Router.AutostartPresets[0] != "keep" {
		t.Fatalf("config=%#v error=%v", document.Parsed.Router, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# keep this comment") || !strings.Contains(string(data), "models_max: 2") {
		t.Fatalf("unrelated config changed: %s", data)
	}
}

type autostartCase struct {
	name        string
	config      string
	preset      string
	enable      bool
	wantErr     error
	wantPresets []string
	wantComment string
}

func runAutostartCase(t *testing.T, tc autostartCase) {
	t.Helper()
	ctx := context.Background()
	path := writeConfig(t, tc.config)
	store := NewFileStore(path, DefaultLimitBytes)
	_, err := store.SetRouterAutostartPreset(ctx, tc.preset, tc.enable)
	if tc.wantErr != nil {
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("expected error %v, got %v", tc.wantErr, err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := store.Read(ctx)
	got := doc.Parsed.Router.AutostartPresets
	if len(got) != len(tc.wantPresets) {
		t.Fatalf("got %v, want %v", got, tc.wantPresets)
	}
	for i, want := range tc.wantPresets {
		if got[i] != want {
			t.Fatalf("presets[%d]: got %q, want %q", i, got[i], want)
		}
	}
	if tc.wantComment != "" {
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), tc.wantComment) {
			t.Fatalf("comment %q lost: %s", tc.wantComment, data)
		}
	}
}

func TestSetRouterAutostartPreset(t *testing.T) {
	cases := []autostartCase{
		{
			name:        "enable adds name",
			config:      "# top comment\nlisten_addr: 127.0.0.1:7000\nrouter:\n  models_max: 2\n",
			preset:      "mypreset",
			enable:      true,
			wantPresets: []string{"mypreset"},
		},
		{
			name:        "enable is idempotent",
			config:      "listen_addr: 127.0.0.1:7000\nrouter:\n  models_max: 2\n  autostart_presets: [mypreset]\n",
			preset:      "mypreset",
			enable:      true,
			wantPresets: []string{"mypreset"},
		},
		{
			name:        "disable removes name",
			config:      "# keep comment\nlisten_addr: 127.0.0.1:7000\nrouter:\n  models_max: 2\n  autostart_presets: [keep, remove]\n",
			preset:      "remove",
			enable:      false,
			wantPresets: []string{"keep"},
			wantComment: "# keep comment",
		},
		{
			name:        "disable is idempotent when absent",
			config:      "listen_addr: 127.0.0.1:7000\nrouter:\n  models_max: 2\n  autostart_presets: [other]\n",
			preset:      "missing",
			enable:      false,
			wantPresets: []string{"other"},
		},
		{
			name:    "over-cap returns ErrAutostartCapExceeded",
			config:  "listen_addr: 127.0.0.1:7000\nrouter:\n  models_max: 1\n  autostart_presets: [full]\n",
			preset:  "second",
			enable:  true,
			wantErr: config.ErrAutostartCapExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { runAutostartCase(t, tc) })
	}
}

func TestSetStartupServicesPreservesDocument(t *testing.T) {
	path := writeConfig(t, "# keep this comment\nlisten_addr: 127.0.0.1:7000\nstartup_services: [control, web]\nrouter:\n  models_max: 1\n")
	store := NewFileStore(path, DefaultLimitBytes)
	if _, err := store.SetStartupServices(context.Background(), []string{config.StartupServiceControl}); err != nil {
		t.Fatal(err)
	}
	doc, err := store.Read(context.Background())
	if err != nil || len(doc.Parsed.StartupServices) != 1 || doc.Parsed.StartupServices[0] != config.StartupServiceControl {
		t.Fatalf("document=%#v error=%v", doc, err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# keep this comment") || !strings.Contains(string(data), "models_max: 1") {
		t.Fatalf("unrelated config changed: %s", data)
	}
}

func TestSetStartupServicesAddsMissingKeyAndValidates(t *testing.T) {
	path := writeConfig(t, "listen_addr: 127.0.0.1:7000\nrouter:\n  models_max: 1\n")
	store := NewFileStore(path, DefaultLimitBytes)
	if _, err := store.SetStartupServices(context.Background(), []string{config.StartupServiceWeb}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if _, err := store.SetStartupServices(context.Background(), []string{"invalid"}); err == nil {
		t.Fatal("expected invalid startup service error")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid mutation changed config")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, dir)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
