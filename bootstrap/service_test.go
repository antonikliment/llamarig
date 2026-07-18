package bootstrap

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"llamarig/config"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"

	"go.uber.org/zap"
)

const testAuthTokenEnv = "TEST_" + config.ProjectTokenEnv

func TestNewGatewayBuildsHTTPServerWithAppFSAndAuth(t *testing.T) {
	configPath := writeBootstrapConfig(t)
	t.Setenv("LLAMARIG_CONFIG", configPath)
	writeBootstrapApp(t)

	gateway, err := NewGateway(context.Background(), Options{
		Logger: zap.NewNop(),
		Env: func(name string) string {
			if name == testAuthTokenEnv {
				return "secret"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("NewGateway returned error: %v", err)
	}
	if gateway.Config.ListenAddr != "127.0.0.1:0" {
		t.Fatalf("listen addr = %q", gateway.Config.ListenAddr)
	}

	assertPublicApp(t, gateway.HTTPServer.Handler)
	assertMCPRequiresAuth(t, gateway.HTTPServer.Handler)
}

func assertPublicApp(t *testing.T, handler http.Handler) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Bootstrap App") {
		t.Fatalf("root status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func assertMCPRequiresAuth(t *testing.T, handler http.Handler) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unauthorized mcp status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestNewServiceBuildsControlRPCSocket(t *testing.T) {
	configPath := writeBootstrapConfig(t)
	t.Setenv("LLAMARIG_CONFIG", configPath)

	svc, err := NewService(context.Background(), Options{Logger: zap.NewNop(), Env: os.Getenv})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	defer cleanupControlRPC(svc)

	socketInfo, err := os.Stat(svc.ControlRPCSocketPath)
	if err != nil {
		t.Fatalf("stat control rpc socket: %v", err)
	}
	if socketInfo.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode = %v", socketInfo.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(svc.ControlRPCSocketPath))
	if err != nil {
		t.Fatalf("stat control rpc socket dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("socket dir mode = %v", dirInfo.Mode().Perm())
	}

	errs := make(chan error, 1)
	go func() {
		errs <- svc.ControlRPCServer.Serve(svc.ControlRPCListener)
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = svc.ControlRPCServer.Shutdown(ctx)
		if err := <-errs; err != nil && err != http.ErrServerClosed {
			t.Fatalf("control rpc serve error: %v", err)
		}
	}()

	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "unix", svc.ControlRPCSocketPath)
	}}}
	rpcClient := controlv1connect.NewControlServiceClient(client, "http://llamarig")
	resp, err := rpcClient.Health(context.Background(), &controlv1.HealthRequest{})
	if err != nil {
		t.Fatalf("control rpc request: %v", err)
	}
	if !resp.GetOk() || resp.GetService() == "" {
		t.Fatalf("internal rpc response ok=%v service=%q", resp.GetOk(), resp.GetService())
	}
}

func cleanupControlRPC(svc *Service) {
	svc.Close()
	if svc.ControlRPCListener != nil {
		_ = svc.ControlRPCListener.Close()
	}
	_ = os.Remove(svc.ControlRPCSocketPath)
}

func writeBootstrapConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, dir)
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, "models.ini"), []byte("[agent]\nmodels-dir = "+dir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := `listen_addr: "127.0.0.1:0"
router:
  default_preset: "agent"
security:
  auth_token_env: "` + testAuthTokenEnv + `"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func writeBootstrapApp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>Bootstrap App</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.ProjectAppDirEnv, dir)
}
