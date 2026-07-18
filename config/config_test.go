package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv(ProjectHomeEnv, home)
	cfg := Default()
	if cfg.ListenAddr != "127.0.0.1:7000" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.Router.StopTimeout == 0 || cfg.Router.ModelsMax != 1 ||
		cfg.Router.ReadinessTimeout != 60*time.Second || cfg.Router.ReadinessInterval != 500*time.Millisecond {
		t.Fatal("expected router defaults")
	}
	if cfg.Router.Host != DefaultLlamaHost {
		t.Fatalf("Router.Host = %q, want %q", cfg.Router.Host, DefaultLlamaHost)
	}
	if cfg.Security.DisableOriginCheck {
		t.Fatal("DisableOriginCheck default = true")
	}
}

func TestParseDefaultsRouterReadiness(t *testing.T) {
	cfg, err := Parse([]byte("listen_addr: 127.0.0.1:7000\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Router.ReadinessTimeout != DefaultReadinessTimeout || cfg.Router.ReadinessInterval != DefaultReadinessInterval {
		t.Fatalf("router readiness defaults = %s, %s", cfg.Router.ReadinessTimeout, cfg.Router.ReadinessInterval)
	}
}

func TestAllowsNonLoopback(t *testing.T) {
	for _, tc := range []struct {
		name string
		addr string
		want bool
	}{
		{name: "loopback ipv4", addr: "127.0.0.1:7000", want: false},
		{name: "loopback ipv6", addr: "[::1]:7000", want: false},
		{name: "localhost", addr: "localhost:7000", want: false},
		{name: "all interfaces ipv4", addr: "0.0.0.0:7000", want: true},
		{name: "all interfaces ipv6", addr: "[::]:7000", want: true},
		{name: "host omitted", addr: ":7000", want: true},
		{name: "hostname", addr: "llamarig.local:7000", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{ListenAddr: tc.addr}
			if got := cfg.AllowsNonLoopback(); got != tc.want {
				t.Fatalf("AllowsNonLoopback() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseHonorsRouterHostOverride(t *testing.T) {
	cfg, err := Parse([]byte("listen_addr: 127.0.0.1:7000\nrouter:\n  host: 0.0.0.0\n  port: 8080\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Router.Host != "0.0.0.0" {
		t.Fatalf("Router.Host = %q, want 0.0.0.0", cfg.Router.Host)
	}
}

func TestRouterAllowsNonLoopback(t *testing.T) {
	for _, tc := range []struct {
		name string
		host string
		want bool
	}{
		{name: "default loopback", host: "127.0.0.1", want: false},
		{name: "localhost", host: "localhost", want: false},
		{name: "loopback ipv6", host: "::1", want: false},
		{name: "all interfaces", host: "0.0.0.0", want: true},
		{name: "lan address", host: "192.168.1.5", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Router: RouterConfig{Host: tc.host}}
			if got := cfg.RouterAllowsNonLoopback(); got != tc.want {
				t.Fatalf("RouterAllowsNonLoopback() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRemoteBindDoesNotFailValidationWithoutToken(t *testing.T) {
	cfg := Default()
	cfg.ListenAddr = "0.0.0.0:7000"
	if err := cfg.Router.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestLoadUsesExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yml")
	if err := os.WriteFile(path, []byte("listen_addr: \"127.0.0.1:7100\"\nrouter:\n  default_preset: \"qwen\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("LLAMARIG_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:7100" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestLoadParsesDisableOriginCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yml")
	if err := os.WriteFile(path, []byte("security:\n  disable_origin_check: true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("LLAMARIG_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Security.DisableOriginCheck {
		t.Fatal("DisableOriginCheck = false")
	}
}

func TestLoadUsesDefaultConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv(ProjectHomeEnv, home)
	t.Setenv("LLAMARIG_CONFIG", "")

	homeConfig := filepath.Join(home, "config.yaml")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home config: %v", err)
	}
	if err := os.WriteFile(homeConfig, []byte("listen_addr: \"127.0.0.1:7101\"\n"), 0o600); err != nil {
		t.Fatalf("write home config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:7101" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
}

func TestConfigPathResolution(t *testing.T) {
	t.Setenv("LLAMARIG_CONFIG", "")
	t.Setenv(ProjectHomeEnv, "/tmp/"+ProjectName+"-test")
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath returned error: %v", err)
	}
	if path != filepath.Join("/tmp/"+ProjectName+"-test", "config.yaml") {
		t.Fatalf("ConfigPath = %q", path)
	}

	t.Setenv("LLAMARIG_CONFIG", "/tmp/custom.yaml")
	path, err = ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath returned error: %v", err)
	}
	if path != "/tmp/custom.yaml" {
		t.Fatalf("ConfigPath = %q", path)
	}
}

func TestResolveModelStorageDir(t *testing.T) {
	home := t.TempDir()
	userHome := t.TempDir()
	t.Setenv(ProjectHomeEnv, home)
	t.Setenv("HOME", userHome)

	path, err := ResolveModelStorageDir("")
	if err != nil {
		t.Fatalf("ResolveModelStorageDir returned error: %v", err)
	}
	if path != filepath.Join(home, "models") {
		t.Fatalf("default model storage dir = %q", path)
	}

	path, err = ResolveModelStorageDir("local-models")
	if err != nil {
		t.Fatalf("ResolveModelStorageDir returned error: %v", err)
	}
	if path != filepath.Join(home, "local-models") {
		t.Fatalf("relative model storage dir = %q", path)
	}

	path, err = ResolveModelStorageDir(filepath.Join(userHome, "hf-models"))
	if err != nil {
		t.Fatalf("ResolveModelStorageDir returned error: %v", err)
	}
	if path != filepath.Join(userHome, "hf-models") {
		t.Fatalf("absolute model storage dir = %q", path)
	}

	path, err = ResolveModelStorageDir("~/hf-models")
	if err != nil {
		t.Fatalf("ResolveModelStorageDir returned error: %v", err)
	}
	if path != filepath.Join(userHome, "hf-models") {
		t.Fatalf("home model storage dir = %q", path)
	}
}

func TestResolveCatalogCacheDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv(ProjectHomeEnv, home)

	path, err := ResolveCatalogCacheDir("")
	if err != nil {
		t.Fatalf("ResolveCatalogCacheDir returned error: %v", err)
	}
	if path != filepath.Join(home, "cache", "hf-catalog") {
		t.Fatalf("default catalog cache dir = %q", path)
	}

	path, err = ResolveCatalogCacheDir("cache/custom")
	if err != nil {
		t.Fatalf("ResolveCatalogCacheDir returned error: %v", err)
	}
	if path != filepath.Join(home, "cache", "custom") {
		t.Fatalf("relative catalog cache dir = %q", path)
	}
}

func TestLogArchiveRetentionDefaultAndDisable(t *testing.T) {
	cfg, err := Parse([]byte("listen_addr: 127.0.0.1:7000\nrouter:\n  port: 8080\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogArchiveRetention != DefaultLogArchiveRetention {
		t.Fatalf("default retention = %s", cfg.LogArchiveRetention)
	}
	cfg, err = Parse([]byte("log_archive_retention: 0s\nrouter:\n  port: 8080\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogArchiveRetention != 0 {
		t.Fatalf("disabled retention = %s", cfg.LogArchiveRetention)
	}
}

func TestParseRejectsLegacyRuntimeConfig(t *testing.T) {
	_, err := Parse([]byte("runtime:\n  servers:\n    - name: old\n      models_dir: /tmp/models\n"))
	if err == nil {
		t.Fatal("expected legacy runtime.servers to fail strict parse")
	}
}

func TestRouterConfigValidation(t *testing.T) {
	for _, cfg := range []RouterConfig{{Port: 0, ModelsMax: 1}, {Port: 8080, ModelsMax: 0}, {Port: 8080, ModelsMax: 1, AutostartPresets: []string{"a", "b"}}} {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate(%#v) returned nil", cfg)
		}
	}
}
