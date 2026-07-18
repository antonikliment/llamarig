package setup

import (
	"bytes"
	"cmp"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"llamarig/config"
	"llamarig/platform/filedoc"

	"gopkg.in/yaml.v3"
)

type Answers struct {
	ListenAddr      string
	AuthTokenEnv    string
	LlamaExecutable string
	LlamaModelsDir  string
	LlamaModelFile  string
	LlamaPort       string
	AutoStart       bool
	StartupServices []string
}

func DefaultAnswers(paths Paths) Answers {
	return Answers{ListenAddr: config.DefaultListenAddr, AuthTokenEnv: config.DefaultAuthTokenEnv, LlamaExecutable: config.DefaultLlamaExecutable, LlamaModelsDir: filepath.Join(paths.Home, "models"), LlamaPort: strconv.Itoa(config.DefaultLlamaPort), StartupServices: config.DefaultStartupServices()}
}

func WriteFiles(paths Paths, answers Answers, force bool) error {
	cfg, err := RenderConfig(paths, answers)
	if err != nil {
		return err
	}
	preset, err := RenderModelsINI(answers)
	if err != nil {
		return err
	}
	if err := ensureWritableSetup(paths, force); err != nil {
		return err
	}
	return writeSetupFiles(paths, cfg, preset, force)
}

func ensureWritableSetup(paths Paths, force bool) error {
	if !force {
		if err := ensureMissing(paths.Config, "config"); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		return fmt.Errorf("create %s home: %w", config.ProjectName, err)
	}
	if err := os.Chmod(paths.Home, 0o700); err != nil {
		return fmt.Errorf("chmod %s home: %w", config.ProjectName, err)
	}
	return nil
}

func ensureMissing(path string, label string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists at %s", label, path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", label, err)
	}
	return nil
}

func writeSetupFiles(paths Paths, cfg string, preset string, force bool) error {
	if err := writeSetupFile(paths.Config, cfg, force); err != nil {
		return err
	}
	return writeSetupFile(paths.ModelsINI, preset, force)
}

// writeSetupFile overwrites an existing file with a timestamped backup when
// force is set (re-running setup); otherwise it only creates a new file.
func writeSetupFile(path string, content string, force bool) error {
	if force {
		if _, err := os.Stat(path); err == nil {
			_, err := filedoc.WriteFile(path, content, filedoc.WriteOptions{Perm: 0o600, Backup: true})
			return err
		}
	}
	return filedoc.AtomicCreate(path, []byte(content), 0o600)
}

func RenderConfig(paths Paths, answers Answers) (string, error) {
	answers.ListenAddr = cmp.Or(answers.ListenAddr, config.DefaultListenAddr)
	answers.AuthTokenEnv = cmp.Or(answers.AuthTokenEnv, config.DefaultAuthTokenEnv)
	if len(answers.StartupServices) == 0 {
		answers.StartupServices = config.DefaultStartupServices()
	}
	answers.LlamaPort = cmp.Or(answers.LlamaPort, strconv.Itoa(config.DefaultLlamaPort))
	if _, err := netSplitHostPort(answers.ListenAddr); err != nil {
		return "", err
	}
	data := map[string]any{
		"ListenAddr":       answers.ListenAddr,
		"ModelsDir":        filepath.Join(paths.Home, "models"),
		"RouterExecutable": answers.LlamaExecutable,
		"RouterPort":       answers.LlamaPort,
		"AuthTokenEnv":     answers.AuthTokenEnv,
		"AutoStart":        answers.AutoStart,
		"StartupServices":  answers.StartupServices,
	}
	var buf bytes.Buffer
	if err := configTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	var cfg config.Config
	oldHome, hadHome := os.LookupEnv(config.ProjectHomeEnv)
	_ = os.Setenv(config.ProjectHomeEnv, paths.Home)
	defer func() {
		if hadHome {
			_ = os.Setenv(config.ProjectHomeEnv, oldHome)
			return
		}
		_ = os.Unsetenv(config.ProjectHomeEnv)
	}()
	if err := yamlUnmarshal(buf.Bytes(), &cfg); err != nil {
		return "", fmt.Errorf("parse generated config: %w", err)
	}
	if err := cfg.Router.Validate(); err != nil {
		return "", fmt.Errorf("validate generated config: %w", err)
	}
	return buf.String(), nil
}

func RenderModelsINI(answers Answers) (string, error) {
	data := map[string]any{"LlamaModelsDir": answers.LlamaModelsDir, "LlamaModelFile": answers.LlamaModelFile}
	var buf bytes.Buffer
	if err := modelsINITemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var configTemplate = template.Must(template.New("config").Funcs(template.FuncMap{"yamlString": yamlString}).Parse(`listen_addr: {{ yamlString .ListenAddr }}
model_storage_dir: {{ yamlString .ModelsDir }}
startup_services:
{{- range .StartupServices }}
  - {{ yamlString . }}
{{- end }}

security:
  auth_token_env: {{ yamlString .AuthTokenEnv }}
  disable_origin_check: false

router:
  executable: {{ yamlString .RouterExecutable }}
  port: {{ .RouterPort }}
  models_max: 1
  default_preset: "default"
  autostart_presets:
{{- if .AutoStart }}
    - "default"
{{- else }} []
{{- end }}
  stop_timeout: 10s
  env: {}
  readiness_timeout: 60s
  readiness_interval: 500ms
`))

func yamlString(value string) (string, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

var modelsINITemplate = template.Must(template.New("models.ini").Parse(`[default]
{{ if .LlamaModelFile }}
model = {{ .LlamaModelFile }}
{{ else }}
models-dir = {{ .LlamaModelsDir }}
{{ end }}
`))

func netSplitHostPort(addr string) (string, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return "", fmt.Errorf("listen address must be host:port: %w", err)
	}
	return addr, nil
}

var yamlUnmarshal = func(data []byte, out any) error {
	return yaml.Unmarshal(data, out)
}
