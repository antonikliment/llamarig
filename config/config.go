package config

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrAutostartCapExceeded is returned by RouterConfig.Validate when
// autostart_presets exceeds router.models_max.
var ErrAutostartCapExceeded = errors.New("autostart limit (models_max) reached")

const (
	ProjectName        = "llamarig"
	ProjectDisplayName = "LlamaRig"
	ProjectHomeEnv     = "LLAMARIG_HOME"
	ProjectHomeDirName = "." + ProjectName
	ProjectTokenEnv    = "LLAMARIG_CONTROL_TOKEN"
	ProjectSocketEnv   = "LLAMARIG_CONTROL_SOCKET"
	ProjectAppDirEnv   = "LLAMARIG_APP_DIR"

	DefaultListenAddr          = "127.0.0.1:7000"
	DefaultAuthTokenEnv        = ProjectTokenEnv
	DefaultCommandTimeout      = 30 * time.Second
	DefaultCatalogCacheTTL     = 6 * time.Hour
	DefaultLogArchiveRetention = 7 * 24 * time.Hour
	DefaultLlamaExecutable     = "llama-server"
	DefaultLlamaHost           = "127.0.0.1"
	DefaultLlamaPort           = 8080
	DefaultReadinessTimeout    = 60 * time.Second
	DefaultReadinessInterval   = 500 * time.Millisecond

	// StartupServiceControl is the internal Unix-socket control daemon ("serve").
	StartupServiceControl = "control"
	// StartupServiceWeb is the public HTTP/GUI/MCP gateway ("gateway").
	StartupServiceWeb = "web"
)

// DefaultStartupServices starts both the control daemon and the web gateway.
func DefaultStartupServices() []string { return []string{StartupServiceControl, StartupServiceWeb} }

type Config struct {
	ListenAddr          string         `yaml:"listen_addr" json:"listen_addr"`
	ModelStorageDir     string         `yaml:"model_storage_dir" json:"model_storage_dir"`
	CatalogCacheDir     string         `yaml:"catalog_cache_dir" json:"catalog_cache_dir"`
	CatalogCacheTTL     time.Duration  `yaml:"catalog_cache_ttl" json:"catalog_cache_ttl"`
	LogArchiveRetention time.Duration  `yaml:"log_archive_retention" json:"log_archive_retention"`
	StartupServices     []string       `yaml:"startup_services" json:"startup_services,omitempty"`
	Router              RouterConfig   `yaml:"router" json:"router"`
	Security            SecurityConfig `yaml:"security" json:"security"`
}

type SecurityConfig struct {
	AuthTokenEnv       string `yaml:"auth_token_env" json:"auth_token_env"`
	DisableOriginCheck bool   `yaml:"disable_origin_check" json:"disable_origin_check"`
}

type RouterConfig struct {
	Executable        string            `yaml:"executable" json:"executable,omitempty"`
	Host              string            `yaml:"host" json:"host,omitempty"`
	Port              int               `yaml:"port" json:"port"`
	ModelsMax         int               `yaml:"models_max" json:"models_max"`
	DefaultPreset     string            `yaml:"default_preset" json:"default_preset,omitempty"`
	AutostartPresets  []string          `yaml:"autostart_presets" json:"autostart_presets,omitempty"`
	StopTimeout       time.Duration     `yaml:"stop_timeout" json:"stop_timeout"`
	Env               map[string]string `yaml:"env" json:"env,omitempty"`
	ReadinessTimeout  time.Duration     `yaml:"readiness_timeout" json:"readiness_timeout,omitempty"`
	ReadinessInterval time.Duration     `yaml:"readiness_interval" json:"readiness_interval,omitempty"`
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	return LoadFile(path)
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

func Parse(data []byte) (Config, error) {
	cfg := Default()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, err
	}
	cfg.applyDefaults()
	if err := cfg.Router.Validate(); err != nil {
		return Config{}, err
	}
	if cfg.LogArchiveRetention < 0 {
		return Config{}, fmt.Errorf("log_archive_retention must not be negative")
	}
	if err := ValidateStartupServices(cfg.StartupServices); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ValidateStartupServices rejects unknown startup service names.
func ValidateStartupServices(services []string) error {
	for _, name := range services {
		if name != StartupServiceControl && name != StartupServiceWeb {
			return fmt.Errorf("unknown startup service %q (want %q or %q)", name, StartupServiceControl, StartupServiceWeb)
		}
	}
	return nil
}

func Default() Config {
	return Config{ListenAddr: DefaultListenAddr, LogArchiveRetention: DefaultLogArchiveRetention, StartupServices: DefaultStartupServices(), Router: RouterConfig{Host: DefaultLlamaHost, Port: DefaultLlamaPort, ModelsMax: 1, StopTimeout: DefaultCommandTimeout, ReadinessTimeout: DefaultReadinessTimeout, ReadinessInterval: DefaultReadinessInterval}, Security: SecurityConfig{AuthTokenEnv: DefaultAuthTokenEnv}}
}

func (c *Config) applyDefaults() {
	c.ListenAddr = cmp.Or(c.ListenAddr, DefaultListenAddr)
	c.Router.Executable = cmp.Or(c.Router.Executable, DefaultLlamaExecutable)
	c.Router.Host = cmp.Or(c.Router.Host, DefaultLlamaHost)
	c.Router.Port = cmp.Or(c.Router.Port, DefaultLlamaPort)
	c.Router.ModelsMax = cmp.Or(c.Router.ModelsMax, 1)
	c.Router.StopTimeout = cmp.Or(c.Router.StopTimeout, DefaultCommandTimeout)
	c.Router.ReadinessTimeout = cmp.Or(c.Router.ReadinessTimeout, DefaultReadinessTimeout)
	c.Router.ReadinessInterval = cmp.Or(c.Router.ReadinessInterval, DefaultReadinessInterval)
	c.CatalogCacheTTL = cmp.Or(c.CatalogCacheTTL, DefaultCatalogCacheTTL)
	c.Security.AuthTokenEnv = cmp.Or(c.Security.AuthTokenEnv, DefaultAuthTokenEnv)
	if c.StartupServices == nil {
		c.StartupServices = DefaultStartupServices()
	}
}

// RouterAllowsNonLoopback reports whether the llama-server router is
// configured to bind a non-loopback host, which exposes its unauthenticated
// API beyond the local machine.
func (c *Config) RouterAllowsNonLoopback() bool {
	return c.Router.Host != "" && c.Router.Host != "127.0.0.1" && c.Router.Host != "localhost" && c.Router.Host != "::1"
}

func (c *Config) AllowsNonLoopback() bool {
	host, _, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		host = c.ListenAddr
		if c.ListenAddr != "" && c.ListenAddr[0] == ':' {
			return true
		}
	}
	if host == "" {
		return true
	}
	if host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

func LlamaRigHome() (string, error) {
	if home := os.Getenv(ProjectHomeEnv); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ProjectHomeDirName), nil
}

// llamaRigPath resolves a path relative to LlamaRigHome().
func llamaRigPath(sub ...string) (string, error) {
	home, err := LlamaRigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, sub...)...), nil
}

func DefaultConfigPath() (string, error) { return llamaRigPath("config.yaml") }

func DefaultModelStorageDir() (string, error) { return llamaRigPath("models") }

func DefaultCatalogCacheDir() (string, error) { return llamaRigPath("cache", "hf-catalog") }

// ControlSocketPath returns the Unix socket path the control daemon listens
// on and that protocol-adapter clients (CLI, TUI) dial.
func ControlSocketPath() (string, error) { return llamaRigPath("run", "control.sock") }

// resolveDir resolves value as a home-relative path, or defaultFn() if unset.
func resolveDir(value string, defaultFn func() (string, error)) (string, error) {
	if value == "" {
		return defaultFn()
	}
	return resolveHomeRelativePath(value)
}

func ResolveModelStorageDir(value string) (string, error) {
	return resolveDir(value, DefaultModelStorageDir)
}

func ResolveCatalogCacheDir(value string) (string, error) {
	return resolveDir(value, DefaultCatalogCacheDir)
}

func resolveHomeRelativePath(value string) (string, error) {
	value = ExpandHome(value)
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	home, err := LlamaRigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, filepath.Clean(value)), nil
}

func ConfigPath() (string, error) {
	if path := os.Getenv("LLAMARIG_CONFIG"); path != "" {
		return path, nil
	}
	return DefaultConfigPath()
}

func (c RouterConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("router.port must be between 1 and 65535")
	}
	if c.ModelsMax < 1 {
		return fmt.Errorf("router.models_max must be at least 1")
	}
	if len(c.AutostartPresets) > c.ModelsMax {
		return fmt.Errorf("router.autostart_presets has %d entries but router.models_max is %d: %w", len(c.AutostartPresets), c.ModelsMax, ErrAutostartCapExceeded)
	}
	return nil
}

func ExpandHome(path string) string {
	if path == "" || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil && path == "~" {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
