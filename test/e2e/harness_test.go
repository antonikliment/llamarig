package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"llamarig/adapters/public_http"
	"llamarig/bootstrap"
	"llamarig/config"
	"llamarig/core/modelpresets"
	"llamarig/core/rpc/gen/v1/controlv1connect"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const e2eAuthTokenEnv = "E2E_" + config.ProjectTokenEnv

var stubBinPath string
var nextTestPort atomic.Int32

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "llamarig-e2e-stub-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	source := filepath.Join(dir, "main.go")
	if err := os.WriteFile(source, []byte(stubSource), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	stubBinPath = filepath.Join(dir, "llamarig-stubserver")
	cmd := exec.Command("go", "build", "-o", stubBinPath, source)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func startService(t *testing.T) controlv1connect.ControlServiceClient {
	t.Helper()
	svc := startControlService(t)
	return controlClient(svc)
}

func controlClient(svc *bootstrap.Service) controlv1connect.ControlServiceClient {
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "unix", svc.ControlRPCSocketPath)
	}}}
	return controlv1connect.NewControlServiceClient(client, "http://llamarig")
}

func startControlService(t *testing.T) *bootstrap.Service {
	t.Helper()

	configPath := writeConfig(t)
	t.Setenv("LLAMARIG_CONFIG", configPath)

	svc, err := bootstrap.NewService(context.Background(), bootstrap.Options{
		Logger: zap.NewNop(),
		Env:    os.Getenv,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	serveControlRPC(t, svc)
	return svc
}

func serveControlRPC(t *testing.T, svc *bootstrap.Service) {
	t.Helper()
	errs := make(chan error, 1)
	go func() {
		errs <- svc.ControlRPCServer.Serve(svc.ControlRPCListener)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		svc.Close()
		_, _ = svc.Manager.StopOperation(ctx, "")
		_ = svc.ControlRPCServer.Shutdown(ctx)
		if err := <-errs; err != nil && err != http.ErrServerClosed {
			t.Fatalf("control rpc serve: %v", err)
		}
		if svc.ControlRPCListener != nil {
			_ = svc.ControlRPCListener.Close()
		}
		_ = os.Remove(svc.ControlRPCSocketPath)
	})
}

func startGateway(t *testing.T) http.Handler {
	t.Helper()
	svc := startControlService(t)
	return public_http.NewServer(public_http.Dependencies{
		InternalSocketPath: svc.ControlRPCSocketPath,
		DisableOriginCheck: true,
	}).Handler()
}

func writeConfig(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, home)

	configPath := filepath.Join(home, "config.yaml")
	modelsDir := filepath.Join(home, "models")
	cacheDir := filepath.Join(home, "catalog-cache")
	for _, dir := range []string{modelsDir, cacheDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	port := os.Getenv("E2E_ROUTER_PORT")
	if port == "" {
		port = fmt.Sprint(freePort(t))
	}
	exe := os.Getenv("E2E_LLAMA_SERVER")
	if exe == "" {
		exe = stubBinPath
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("invalid E2E_ROUTER_PORT %q: %v", port, err)
	}
	content, err := renderE2EConfig(modelsDir, cacheDir, exe, filepath.Dir(exe), portNumber)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func renderE2EConfig(modelsDir, cacheDir, exe, libraryDir string, port int) ([]byte, error) {
	return yaml.Marshal(map[string]any{
		"listen_addr":       "127.0.0.1:0",
		"model_storage_dir": modelsDir,
		"catalog_cache_dir": cacheDir,
		"catalog_cache_ttl": "1m",
		"router": map[string]any{
			"port":              port,
			"models_max":        2,
			"default_preset":    "agent",
			"executable":        exe,
			"readiness_timeout": "30s",
			"env": map[string]string{
				"LD_LIBRARY_PATH":   libraryDir,
				"DYLD_LIBRARY_PATH": libraryDir,
			},
		},
		"security": map[string]any{"auth_token_env": e2eAuthTokenEnv},
	})
}

func TestRenderE2EConfigEscapesWindowsPaths(t *testing.T) {
	modelsDir := `C:\Users\test\models`
	cacheDir := `C:\Users\test\catalog-cache`
	exe := `C:\Program Files\llama.cpp\llama-server.exe`
	libraryDir := `C:\Program Files\llama.cpp`
	content, err := renderE2EConfig(modelsDir, cacheDir, exe, libraryDir, 8080)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(content)
	if err != nil {
		t.Fatalf("parse config: %v\n%s", err, content)
	}
	if cfg.ModelStorageDir != modelsDir || cfg.CatalogCacheDir != cacheDir || cfg.Router.Executable != exe || cfg.Router.Env["LD_LIBRARY_PATH"] != libraryDir {
		t.Fatalf("config paths = %#v", cfg)
	}
}

func writePreset(t *testing.T, name string, entries map[string]string) {
	t.Helper()

	home, err := config.LlamaRigHome()
	if err != nil {
		t.Fatal(err)
	}
	store := modelpresets.NewStore(filepath.Join(home, "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: name, Values: entries}, true); err != nil {
		t.Fatal(err)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	return int(nextTestPort.Add(1) + 18080)
}

func stubPresetEntries(t *testing.T) map[string]string {
	t.Helper()

	modelsDir := filepath.Join(t.TempDir(), "models")
	if err := os.MkdirAll(modelsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"models-dir": modelsDir}
}

func requireOK(t *testing.T, label string, ok bool) {
	t.Helper()
	if !ok {
		t.Fatalf("%s ok=false", label)
	}
}

const stubSource = `package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	port := "8080"
	for i, arg := range os.Args[:len(os.Args)-1] {
		if arg == "--port" {
			port = os.Args[i+1]
		}
	}
	mux := http.NewServeMux()
	loaded := map[string]bool{}
	var mu sync.Mutex
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /models", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		data := make([]map[string]any, 0, len(loaded))
		for name := range loaded {
			data = append(data, map[string]any{"id": name, "status": map[string]string{"value": "loaded"}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	change := func(load bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var body struct{ Model string ` + "`json:\"model\"`" + ` }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model == "" {
				http.Error(w, "model required", http.StatusBadRequest)
				return
			}
			mu.Lock()
			if load { loaded[body.Model] = true } else { delete(loaded, body.Model) }
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		}
	}
	mux.HandleFunc("POST /models/load", change(true))
	mux.HandleFunc("POST /models/unload", change(false))
	server := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = server.Close()
}
`
