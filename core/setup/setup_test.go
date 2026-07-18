package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"llamarig/config"
)

func TestEnsureSkipsWhenEnvConfigSet(t *testing.T) {
	t.Setenv("LLAMARIG_CONFIG", "/tmp/custom.yaml")
	err := EnsureWithOptions(context.Background(), Options{
		IsTerminal: func(int) bool { return false },
		RunWizard: func(context.Context, Paths) (Answers, error) {
			t.Fatal("wizard should not run")
			return Answers{}, nil
		},
	})
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
}

func TestEnsureNonInteractiveMissingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LLAMARIG_CONFIG", "")
	t.Setenv(config.ProjectHomeEnv, dir)
	err := EnsureWithOptions(context.Background(), Options{IsTerminal: func(int) bool { return false }})
	if err == nil || !strings.Contains(err.Error(), "no "+config.ProjectDisplayName+" config found") {
		t.Fatalf("Ensure error = %v", err)
	}
}

func TestEnsureWritesFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LLAMARIG_CONFIG", "")
	t.Setenv(config.ProjectHomeEnv, dir)
	err := EnsureWithOptions(context.Background(), Options{
		IsTerminal: func(int) bool { return true },
		RunWizard: func(_ context.Context, paths Paths) (Answers, error) {
			answers := DefaultAnswers(paths)
			return answers, nil
		},
	})
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, "config.yaml"), 0o600)
	assertMode(t, filepath.Join(dir, "models.ini"), 0o600)
}

func TestEnsureSkipsExistingConfigWithoutForce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LLAMARIG_CONFIG", "")
	t.Setenv(config.ProjectHomeEnv, dir)
	wizardRuns := 0
	run := func(_ context.Context, paths Paths) (Answers, error) {
		wizardRuns++
		return DefaultAnswers(paths), nil
	}
	if err := EnsureWithOptions(context.Background(), Options{IsTerminal: func(int) bool { return true }, RunWizard: run}); err != nil {
		t.Fatalf("first Ensure returned error: %v", err)
	}
	if err := EnsureWithOptions(context.Background(), Options{IsTerminal: func(int) bool { return true }, RunWizard: run}); err != nil {
		t.Fatalf("second Ensure returned error: %v", err)
	}
	if wizardRuns != 1 {
		t.Fatalf("wizardRuns = %d, want 1 (second Ensure should skip)", wizardRuns)
	}
}

func TestRerunOverwritesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LLAMARIG_CONFIG", "")
	t.Setenv(config.ProjectHomeEnv, dir)
	configPath := filepath.Join(dir, "config.yaml")
	run := func(_ context.Context, paths Paths) (Answers, error) {
		return DefaultAnswers(paths), nil
	}
	if err := EnsureWithOptions(context.Background(), Options{IsTerminal: func(int) bool { return true }, RunWizard: run}); err != nil {
		t.Fatalf("first Ensure returned error: %v", err)
	}
	if err := EnsureWithOptions(context.Background(), Options{Force: true, IsTerminal: func(int) bool { return true }, RunWizard: run}); err != nil {
		t.Fatalf("forced Ensure returned error: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("stat config after forced run: %v", err)
	}
	matches, err := filepath.Glob(configPath + ".backup-*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup files = %v, want exactly 1", matches)
	}
}

func TestEnsureCancelledWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LLAMARIG_CONFIG", "")
	t.Setenv(config.ProjectHomeEnv, dir)
	err := EnsureWithOptions(context.Background(), Options{
		IsTerminal: func(int) bool { return true },
		RunWizard: func(context.Context, Paths) (Answers, error) {
			return Answers{}, ErrCancelled
		},
	})
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("Ensure error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("config stat error = %v", err)
	}
}

func TestRenderConfigUsesSelectedRuntime(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{Home: dir, Config: filepath.Join(dir, "config.yaml"), ModelsINI: filepath.Join(dir, "models.ini")}
	answers := DefaultAnswers(paths)
	answers.AutoStart = true
	rendered, err := RenderConfig(paths, answers)
	if err != nil {
		t.Fatalf("RenderConfig returned error: %v", err)
	}
	if !strings.Contains(rendered, `autostart_presets:`) || !strings.Contains(rendered, `default_preset: "default"`) {
		t.Fatalf("rendered config = %s", rendered)
	}
	base, err := RenderModelsINI(answers)
	if err != nil {
		t.Fatalf("RenderModelsINI returned error: %v", err)
	}
	if !strings.Contains(base, `models-dir =`) {
		t.Fatalf("rendered base = %s", base)
	}
}

func TestRenderConfigEscapesWindowsPaths(t *testing.T) {
	home := `C:\Users\test\LlamaRig Home`
	paths := Paths{Home: home, Config: home + `\config.yaml`, ModelsINI: home + `\models.ini`}
	answers := DefaultAnswers(paths)
	answers.LlamaExecutable = `C:\Program Files\llama.cpp\llama-server.exe`

	rendered, err := RenderConfig(paths, answers)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse([]byte(rendered))
	if err != nil {
		t.Fatalf("parse rendered config: %v\n%s", err, rendered)
	}
	if cfg.ModelStorageDir != filepath.Join(home, "models") || cfg.Router.Executable != answers.LlamaExecutable {
		t.Fatalf("rendered paths = %q, %q", cfg.ModelStorageDir, cfg.Router.Executable)
	}
}

func TestWizardModelNavigation(t *testing.T) {
	m := newModel(Paths{Home: "/tmp/" + config.ProjectName, Config: "/tmp/" + config.ProjectName + "/config.yaml"})
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(*model)
	if m.step != stepListen {
		t.Fatalf("step = %v", m.step)
	}
	m.input.SetValue("bad")
	next, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(*model)
	if m.step != stepListen || m.err == nil {
		t.Fatalf("expected listen validation error, step=%v err=%v", m.step, m.err)
	}
}

func TestWizardModelLlamaAutostartQuestion(t *testing.T) {
	llamaExe := filepath.Join(t.TempDir(), "llama-server")
	if err := os.WriteFile(llamaExe, nil, 0o755); err != nil {
		t.Fatalf("write fake llama-server: %v", err)
	}
	m := newModel(Paths{Home: "/tmp/" + config.ProjectName, Config: "/tmp/" + config.ProjectName + "/config.yaml"})
	for _, value := range []string{"", "127.0.0.1:7000", config.ProjectTokenEnv, llamaExe, "/tmp/models", "", "127.0.0.1", "8080"} {
		m.input.SetValue(value)
		next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		m = next.(*model)
	}
	if m.step != stepAutoStart {
		t.Fatalf("step = %v, want autostart", m.step)
	}
	m.input.SetValue("yes")
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(*model)
	if !m.answers.AutoStart || m.step != stepStartupServices {
		t.Fatalf("autostart=%v step=%v", m.answers.AutoStart, m.step)
	}
	m.input.SetValue("both")
	next, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(*model)
	if m.step != stepReview {
		t.Fatalf("step=%v, want review", m.step)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
