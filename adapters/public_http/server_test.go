package public_http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"llamarig/core/rpc"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"

	"llamarig/config"
	"llamarig/core/configstore"
	"llamarig/core/control"
	"llamarig/core/modelcatalog"
	"llamarig/core/modeldownload"
	"llamarig/core/modelpresets"
	"llamarig/core/router"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
	"llamarig/core/runtime"
	"llamarig/core/signals"
)

func TestServerHealthzFlow(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{}, Dependencies{})
	for _, target := range []string{"/health", "/api/health"} {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()

			server.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["ok"] != true || body["service"] != rpc.ServiceName {
				t.Fatalf("body = %#v", body)
			}
		})
	}
}

func TestServerHealthCallsInternalRPCSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	path, handler := controlv1connect.NewControlServiceHandler(fakeControlRPC{})
	internalServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, path) {
			t.Fatalf("unexpected rpc path = %q", r.URL.Path)
		}
		handler.ServeHTTP(w, r)
	})}
	errs := make(chan error, 1)
	go func() {
		errs <- internalServer.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = internalServer.Close()
		err := <-errs
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("internal server: %v", err)
		}
	})

	server := NewServer(Dependencies{InternalSocketPath: socketPath})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true || body["service"] != "socket-rpc" {
		t.Fatalf("body = %#v", body)
	}
}

type fakeControlRPC struct {
	controlv1connect.UnimplementedControlServiceHandler
}

func (fakeControlRPC) Health(context.Context, *controlv1.HealthRequest) (*controlv1.HealthResponse, error) {
	return &controlv1.HealthResponse{Ok: true, Service: "socket-rpc"}, nil
}

type nilControlClient struct{}

func (nilControlClient) Health(context.Context, *controlv1.HealthRequest) (*controlv1.HealthResponse, error) {
	return nil, nil
}

func (nilControlClient) GetInfo(context.Context, *controlv1.GetInfoRequest) (*controlv1.GetInfoResponse, error) {
	return nil, nil
}

func (nilControlClient) GetRuntimeStatus(context.Context, *controlv1.GetRuntimeStatusRequest) (*controlv1.GetRuntimeStatusResponse, error) {
	return nil, nil
}

func (nilControlClient) GetRuntimeResources(context.Context, *controlv1.GetRuntimeResourcesRequest) (*controlv1.GetRuntimeResourcesResponse, error) {
	return nil, nil
}

func (nilControlClient) GetLlamaServerParams(context.Context, *controlv1.GetLlamaServerParamsRequest) (*controlv1.GetLlamaServerParamsResponse, error) {
	return nil, nil
}

func (nilControlClient) StartRuntime(context.Context, *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return nil, nil
}

func (nilControlClient) StopRuntime(context.Context, *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return nil, nil
}

func (nilControlClient) RestartRuntime(context.Context, *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return nil, nil
}

func (nilControlClient) GetSignals(context.Context, *controlv1.GetSignalsRequest) (*controlv1.GetSignalsResponse, error) {
	return nil, nil
}

func (nilControlClient) ListEvents(context.Context, *controlv1.ListEventsRequest) (*controlv1.ListEventsResponse, error) {
	return nil, nil
}

func (nilControlClient) WatchEvents(context.Context, *controlv1.WatchEventsRequest) (*connect.ServerStreamForClient[controlv1.Event], error) {
	return nil, nil
}

func (nilControlClient) GetConfig(context.Context, *controlv1.GetConfigRequest) (*controlv1.TextDocumentResponse, error) {
	return nil, nil
}

func (nilControlClient) ValidateConfig(context.Context, *controlv1.ValidateTextDocumentRequest) (*controlv1.ValidationResponse, error) {
	return nil, nil
}

func (nilControlClient) ReplaceConfig(context.Context, *controlv1.ReplaceTextDocumentRequest) (*controlv1.MutationResponse, error) {
	return nil, nil
}

func (nilControlClient) ListPresets(context.Context, *controlv1.ListPresetsRequest) (*controlv1.ListPresetsResponse, error) {
	return nil, nil
}

func (nilControlClient) GetPreset(context.Context, *controlv1.GetPresetRequest) (*controlv1.PresetResponse, error) {
	return nil, nil
}

func (nilControlClient) PutPreset(context.Context, *controlv1.PutPresetRequest) (*controlv1.PresetResponse, error) {
	return nil, nil
}

func (nilControlClient) DeletePreset(context.Context, *controlv1.DeletePresetRequest) (*controlv1.MutationResponse, error) {
	return nil, nil
}

func (nilControlClient) CleanupPreset(context.Context, *controlv1.CleanupPresetRequest) (*controlv1.MutationResponse, error) {
	return nil, nil
}

func (nilControlClient) ResolveModel(context.Context, *controlv1.ResolveModelRequest) (*controlv1.ResolveModelResponse, error) {
	return nil, nil
}

func (nilControlClient) ListModelCatalog(context.Context, *controlv1.ListModelCatalogRequest) (*controlv1.ListModelCatalogResponse, error) {
	return nil, nil
}

func (nilControlClient) ListLocalModels(context.Context, *controlv1.ListLocalModelsRequest) (*controlv1.ListLocalModelsResponse, error) {
	return nil, nil
}

func (nilControlClient) DeleteLocalModel(context.Context, *controlv1.DeleteLocalModelRequest) (*controlv1.MutationResponse, error) {
	return nil, nil
}

func (nilControlClient) WatchModelCatalog(context.Context, *controlv1.WatchModelCatalogRequest) (*connect.ServerStreamForClient[controlv1.ModelCatalogEvent], error) {
	return nil, nil
}

func (nilControlClient) StartModelDownload(context.Context, *controlv1.StartModelDownloadRequest) (*controlv1.ModelDownloadResponse, error) {
	return nil, nil
}

func (nilControlClient) GetModelDownload(context.Context, *controlv1.GetModelDownloadRequest) (*controlv1.ModelDownloadResponse, error) {
	return nil, nil
}

func (nilControlClient) CancelModelDownload(context.Context, *controlv1.CancelModelDownloadRequest) (*controlv1.ModelDownloadResponse, error) {
	return nil, nil
}

func (nilControlClient) ApplyModelDownloadToPreset(context.Context, *controlv1.ApplyModelDownloadToPresetRequest) (*controlv1.MutationResponse, error) {
	return nil, nil
}

func (nilControlClient) SetPresetAutostart(context.Context, *controlv1.PresetAutostartRequest) (*controlv1.MutationResponse, error) {
	return nil, nil
}

func (fakeControlRPC) GetConfig(context.Context, *controlv1.GetConfigRequest) (*controlv1.TextDocumentResponse, error) {
	return &controlv1.TextDocumentResponse{Ok: true, Content: "listen_addr: 127.0.0.1:9999\n"}, nil
}

func TestHTTPNilRPCResponsesReturnRuntimeErrors(t *testing.T) {
	server := NewServer(Dependencies{})
	server.internalControl = nilControlClient{}

	tests := []struct {
		method string
		target string
		body   string
	}{
		{method: http.MethodGet, target: "/health"},
		{method: http.MethodGet, target: "/info"},
		{method: http.MethodGet, target: "/api/runtime/status"},
		{method: http.MethodGet, target: "/api/signals"},
		{method: http.MethodGet, target: "/api/events"},
		{method: http.MethodGet, target: "/api/presets"},
		{method: http.MethodGet, target: "/api/presets/qwen"},
		{method: http.MethodPost, target: "/api/models/resolve", body: `{"url":"https://example.invalid/model.gguf"}`},
		{method: http.MethodGet, target: "/api/models/catalog"},
		{method: http.MethodPost, target: "/api/models/downloads/job/apply-to-preset", body: `{"preset":"qwen"}`},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["ok"] != false {
				t.Fatalf("body = %#v", body)
			}
		})
	}
}

func TestServerServesAppAtRoot(t *testing.T) {
	appFS := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>LlamaRig Control</title>")},
		"app.js":     {Data: []byte(`console.log("` + config.ProjectName + `")`)},
	}
	server := NewServer(Dependencies{AppFS: appFS})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("root status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "LlamaRig Control") {
		t.Fatalf("root body = %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("app.js status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), config.ProjectName) {
		t.Fatalf("app.js body = %q", rec.Body.String())
	}
}

func TestServerAppDoesNotShadowAPI(t *testing.T) {
	appFS := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>LlamaRig Control</title>")},
	}
	server := newHTTPTestServer(t, rpc.RPCDependencies{}, Dependencies{AppFS: appFS})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("api status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("body = %#v", body)
	}
}

func TestServerMissingAppAssetReturnsNotFound(t *testing.T) {
	appFS := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>LlamaRig Control</title>")},
	}
	server := NewServer(Dependencies{AppFS: appFS})

	req := httptest.NewRequest(http.MethodGet, "/missing.js", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPPresetsFlow(t *testing.T) {
	root := writeHTTPPreset(t, "qwen", map[string]string{"model": "/models/qwen.gguf", "ctx-size": "4096"})
	manager := newHTTPManager(control.Dependencies{
		Presets: modelpresets.NewStore(root),
	})
	server := newHTTPTestServer(t, rpc.RPCDependencies{Manager: manager}, Dependencies{AuthToken: "secret"})

	assertStatus(t, server, http.MethodGet, "/api/presets", nil, http.StatusOK)
	rec := assertStatus(t, server, http.MethodGet, "/api/presets/qwen", nil, http.StatusOK)
	var presetBody struct {
		Preset struct {
			Name    string `json:"name"`
			Entries []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"entries"`
		} `json:"preset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &presetBody); err != nil {
		t.Fatalf("decode preset: %v", err)
	}
	if presetBody.Preset.Name != "qwen" || presetBody.Preset.Entries[0].Key != "model" {
		t.Fatalf("preset = %#v", presetBody.Preset)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/presets/qwen", bytes.NewBufferString(`{"entries":[{"key":"model","value":"/models/other.gguf"}]}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace preset status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPInfoFlow(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{
		Manager: newHTTPManager(control.Dependencies{}),
	}, Dependencies{})
	for _, target := range []string{"/info", "/api/info"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", target, rec.Code, rec.Body.String())
		}
		var body struct {
			control.RuntimeInfo
			Build *controlv1.BuildInfo `json:"build"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Router.Status != "stopped" || body.Router.CheckedAt == "" {
			t.Fatalf("router = %#v", body.Router)
		}
		if body.Build.GetVersion() == "" {
			t.Fatal("build version is empty")
		}
	}
}

func TestHTTPEventsFlow(t *testing.T) {
	events := control.NewEventStore(10)
	events.Record(context.Background(), control.AuditEvent{Action: "start", Success: true})
	server := newHTTPTestServer(t, rpc.RPCDependencies{Events: events}, Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"action":"start"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPMapsCoreErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "models.ini")
	manager := newHTTPManager(control.Dependencies{
		Presets: modelpresets.NewStore(root),
	})
	server := newHTTPTestServer(t, rpc.RPCDependencies{Manager: manager}, Dependencies{})

	req := httptest.NewRequest(http.MethodPost, "/api/presets", bytes.NewBufferString(`{"name":"bad]name","entries":[{"key":"model","value":"/m.gguf"}]}`))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		OK    bool `json:"ok"`
		Error struct {
			Kind string `json:"kind"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.OK || body.Error.Kind != string(control.ErrorInvalidInput) {
		t.Fatalf("body = %#v", body)
	}
}

func TestHTTPRejectsPresetINIInjection(t *testing.T) {
	for _, body := range []string{
		`{"name":"bad-key","entries":[{"key":"model\n[other]","value":"/m.gguf"}]}`,
		`{"name":"bad-value","entries":[{"key":"model","value":"/m.gguf\n[other]\nmodel = /other.gguf"}]}`,
		`{"name":"bad-case","entries":[{"key":"Model","value":"/m.gguf"}]}`,
	} {
		root := filepath.Join(t.TempDir(), "models.ini")
		manager := newHTTPManager(control.Dependencies{Presets: modelpresets.NewStore(root)})
		server := newHTTPTestServer(t, rpc.RPCDependencies{Manager: manager}, Dependencies{})
		req := httptest.NewRequest(http.MethodPost, "/api/presets", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"kind":"invalid_input"`) {
			t.Fatalf("body=%s status=%d response=%s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestHTTPDeleteMissingPresetIsSuccessful(t *testing.T) {
	root := filepath.Join(t.TempDir(), "models.ini")
	manager := newHTTPManager(control.Dependencies{Presets: modelpresets.NewStore(root)})
	server := newHTTPTestServer(t, rpc.RPCDependencies{Manager: manager}, Dependencies{})
	req := httptest.NewRequest(http.MethodDelete, "/api/presets/missing", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d response=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPRejectsUntrustedOrigin(t *testing.T) {
	server := NewServer(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "127.0.0.1:7000"
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHTTPAllowsMatchingRemoteOrigin(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{}, Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "192.168.1.50:7000"
	req.Header.Set("Origin", "http://192.168.1.50:7000")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHTTPAllowsMatchingBracketedIPv6Origin(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{}, Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "[::1]"
	req.Header.Set("Origin", "http://[::1]")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHTTPAllowsHTTPSOriginWithoutExplicitPort(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{}, Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "127.0.0.1"
	req.TLS = &tls.ConnectionState{}
	req.Header.Set("Origin", "https://127.0.0.1")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHTTPRejectsMatchingUntrustedHostnameOrigin(t *testing.T) {
	server := NewServer(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "rebind.attacker.test:7000"
	req.Header.Set("Origin", "http://rebind.attacker.test:7000")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHTTPRejectsSameHostDifferentOriginPort(t *testing.T) {
	server := NewServer(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "127.0.0.1:7000"
	req.Header.Set("Origin", "http://127.0.0.1:7001")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHTTPAllowsUntrustedOriginWhenDisabled(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{}, Dependencies{DisableOriginCheck: true})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "127.0.0.1:7000"
	req.Header.Set("Origin", "http://remote.example")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHTTPRejectsOversizeTextBody(t *testing.T) {
	server := NewServer(Dependencies{AuthToken: "secret"})
	body := strings.NewReader(strings.Repeat("x", int(requestBodyLimitBytes)+1))
	req := httptest.NewRequest(http.MethodPost, "/api/models/resolve", body)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPRejectsMissingRequestBody(t *testing.T) {
	server := NewServer(Dependencies{AuthToken: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/models/resolve", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Body = nil
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPConfigAndPresetsFlow(t *testing.T) {
	configPath := writeHTTPConfig(t, "router:\n  default_preset: qwen\n")
	modelsDir := t.TempDir()
	root := writeHTTPPresets(t, map[string]map[string]string{
		"default": {"models-dir": modelsDir},
		"qwen":    {"models-dir": modelsDir},
	})
	manager := newHTTPManager(control.Dependencies{
		Config:       configstore.NewFileStore(configPath, configstore.DefaultLimitBytes),
		Presets:      modelpresets.NewStore(root),
		RouterConfig: config.RouterConfig{Port: 8080, ModelsMax: 1, DefaultPreset: "qwen"},
	})
	server := newHTTPTestServer(t, rpc.RPCDependencies{Manager: manager}, Dependencies{AuthToken: "secret"})

	assertStatus(t, server, http.MethodGet, "/api/presets", nil, http.StatusOK)

	req := httptest.NewRequest(http.MethodPost, "/api/runtime/start?preset=default", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPRuntimeStopWithTargetStopsOnePreset(t *testing.T) {
	modelsDir := t.TempDir()
	embedding := filepath.Join(modelsDir, "embedding.gguf")
	if err := os.WriteFile(embedding, []byte("gguf"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := writeHTTPPresets(t, map[string]map[string]string{
		"qwen":       {"models-dir": modelsDir},
		"embeddings": {"model": embedding},
	})
	manager := newHTTPManager(control.Dependencies{
		Presets: modelpresets.NewStore(root),
	})
	server := newHTTPTestServer(t, rpc.RPCDependencies{Manager: manager}, Dependencies{AuthToken: "secret"})

	for _, target := range []string{"/api/runtime/start?preset=qwen", "/api/runtime/start?preset=embeddings"} {
		req := httptest.NewRequest(http.MethodPost, target, nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", target, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/runtime/stop?preset=qwen", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stop status = %d body=%s", rec.Code, rec.Body.String())
	}
	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(status.Presets) != 1 || status.Presets[0].Name != "embeddings" {
		t.Fatalf("presets after targeted stop = %#v", status.Presets)
	}
}

func TestHTTPAuthProtectsSensitiveEndpoints(t *testing.T) {
	root := writeHTTPPreset(t, "qwen", map[string]string{"models-dir": "./models"})
	manager := newHTTPManager(control.Dependencies{
		Presets: modelpresets.NewStore(root),
	})
	server := newHTTPTestServer(t, rpc.RPCDependencies{Manager: manager}, Dependencies{AuthToken: "secret"})

	assertStatus(t, server, http.MethodGet, "/api/runtime/status", nil, http.StatusOK)
	assertStatus(t, server, http.MethodGet, "/api/presets", nil, http.StatusOK)
	assertStatus(t, server, http.MethodPost, "/api/runtime/start", nil, http.StatusForbidden)
	assertStatus(t, server, http.MethodPost, "/api/models/resolve", bytes.NewBufferString(`{"url":"https://huggingface.co/owner/repo"}`), http.StatusForbidden)
}

func TestHTTPModelResolveFlow(t *testing.T) {
	catalog := &fakeModelCatalog{resolution: modelcatalog.Resolution{OK: true, Params: 123}}
	server := newHTTPTestServer(t, rpc.RPCDependencies{ModelCatalog: catalog}, Dependencies{})

	req := httptest.NewRequest(http.MethodPost, "/api/models/resolve", bytes.NewBufferString(`{"url":"https://huggingface.co/owner/repo"}`))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d body=%s", rec.Code, rec.Body.String())
	}
	if catalog.rawURL != "https://huggingface.co/owner/repo" {
		t.Fatalf("catalog rawURL = %q", catalog.rawURL)
	}
	if strings.Contains(rec.Body.String(), `"params":"`) {
		t.Fatalf("64-bit values must remain JSON numbers: %s", rec.Body.String())
	}
}

func TestHTTPModelCatalogList(t *testing.T) {
	discoverer := &fakeModelDiscoverer{result: modelcatalog.ListResult{
		OK:      true,
		Machine: modelcatalog.MachineProfile{AvailableRAMBytes: 123},
		Models: []modelcatalog.CatalogModel{{
			ID:    "owner/repo",
			Owner: "owner",
			Repo:  "repo",
			URL:   "https://huggingface.co/owner/repo",
		}},
	}}
	server := newHTTPTestServer(t, rpc.RPCDependencies{
		ModelDiscoverer: discoverer,
		Machine:         fakeMachineCollector{snapshot: signals.MachineSnapshot{Memory: signals.MemoryStats{AvailableBytes: 123}}},
	}, Dependencies{AuthToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/models/catalog?limit=12&sort=trending&search=qwen&min_fit=marginal", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog status = %d body=%s", rec.Code, rec.Body.String())
	}
	if discoverer.req.Limit != 12 || discoverer.req.Sort != "trending" || discoverer.req.Search != "qwen" || discoverer.req.MinFit != "marginal" {
		t.Fatalf("request = %#v", discoverer.req)
	}
	if discoverer.machine.AvailableRAMBytes != 123 {
		t.Fatalf("machine = %#v", discoverer.machine)
	}
	var payload struct {
		Models  []modelcatalog.CatalogModel `json:"models"`
		Machine modelcatalog.MachineProfile `json:"machine"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Models) != 1 || payload.Models[0].ID != "owner/repo" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.Machine.AvailableRAMBytes != 123 {
		t.Fatalf("machine = %#v", payload.Machine)
	}
}

func TestHTTPModelCatalogListDoesNotRequireAuth(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{ModelDiscoverer: &fakeModelDiscoverer{}}, Dependencies{AuthToken: "secret"})
	assertStatus(t, server, http.MethodGet, "/api/models/catalog", nil, http.StatusOK)
}

func TestHTTPSignalsDoesNotRequireAuth(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{Signals: fakeSignalsCollector{snapshot: signals.Snapshot{
		Memory: signals.MemoryStats{AvailableBytes: 123},
		CPU:    signals.CPUStats{LogicalCores: 8},
	}}}, Dependencies{AuthToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/signals", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("signals status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Signals signals.Snapshot `json:"signals"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Signals.Memory.AvailableBytes != 123 || payload.Signals.CPU.LogicalCores != 8 {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestHTTPRemovedAppRoutesReturnNotFound(t *testing.T) {
	server := NewServer(Dependencies{})
	for _, target := range []string{"/api/runtime/resources", "/api/config.yaml", "/api/config.yaml/validate"} {
		assertStatus(t, server, http.MethodGet, target, nil, http.StatusNotFound)
	}
}

func TestHTTPModelDownloadRequiresAuth(t *testing.T) {
	server := newHTTPTestServer(t, rpc.RPCDependencies{ModelDownloader: &fakeModelDownloader{}}, Dependencies{AuthToken: "secret"})
	assertStatus(t, server, http.MethodPost, "/api/models/downloads", bytes.NewBufferString(`{"url":"https://huggingface.co/owner/repo","filename":"model.gguf"}`), http.StatusForbidden)
}

func TestHTTPModelApplyToPreset(t *testing.T) {
	target := filepath.Join(t.TempDir(), "model.gguf")
	root := writeHTTPPreset(t, "qwen", map[string]string{"models-dir": "./models"})
	manager := newHTTPManager(control.Dependencies{Presets: modelpresets.NewStore(root)})
	downloader := &fakeModelDownloader{job: modeldownload.Job{
		ID:         "dl_test",
		State:      modeldownload.StateCompleted,
		Filename:   "model.gguf",
		TargetPath: target,
	}}
	server := newHTTPTestServer(t, rpc.RPCDependencies{
		Manager:         manager,
		ModelDownloader: downloader,
	}, Dependencies{AuthToken: "secret"})

	req := httptest.NewRequest(http.MethodPost, "/api/models/downloads/dl_test/apply-to-preset", bytes.NewBufferString(`{"preset":"qwen","restart":false}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d body=%s", rec.Code, rec.Body.String())
	}
	section, err := manager.GetPreset(context.Background(), "qwen")
	if err != nil {
		t.Fatalf("GetPreset returned error: %v", err)
	}
	if section.Values["model"] != target {
		t.Fatalf("model entry = %q", section.Values["model"])
	}
	if _, ok := section.Values["models-dir"]; ok {
		t.Fatalf("models-dir not cleared: %#v", section.Values)
	}
}

func TestHTTPModelApplyPreviewDoesNotMutatePreset(t *testing.T) {
	target := filepath.Join(t.TempDir(), "model.gguf")
	root := writeHTTPPreset(t, "qwen", map[string]string{"models-dir": "./models"})
	manager := newHTTPManager(control.Dependencies{Presets: modelpresets.NewStore(root)})
	downloader := &fakeModelDownloader{job: modeldownload.Job{ID: "dl_test", State: modeldownload.StateCompleted, TargetPath: target}}
	server := newHTTPTestServer(t, rpc.RPCDependencies{
		Manager:         manager,
		ModelDownloader: downloader,
	}, Dependencies{AuthToken: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/models/downloads/dl_test/apply-to-preset", bytes.NewBufferString(`{"preset":"qwen","preview":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"updated"`) {
		t.Fatalf("preview status = %d body=%s", rec.Code, rec.Body.String())
	}
	section, err := manager.GetPreset(context.Background(), "qwen")
	if err != nil {
		t.Fatalf("GetPreset returned error: %v", err)
	}
	if section.Values["models-dir"] == "" {
		t.Fatalf("preset mutated: %#v", section.Values)
	}
}

func TestMCPAuthProtectsEndpoint(t *testing.T) {
	server := NewServer(Dependencies{AuthToken: "secret"})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unauthorized mcp status = %d", rec.Code)
	}
}

func newHTTPTestServer(t *testing.T, rpcDeps rpc.RPCDependencies, httpDeps Dependencies) *Server {
	t.Helper()
	if rpcDeps.Manager == nil {
		rpcDeps.Manager = newHTTPManager(control.Dependencies{})
	}
	return newHTTPTestServerWithControlHandler(t, rpc.NewControlService(rpcDeps), httpDeps)
}

func newHTTPTestServerWithControlHandler(t *testing.T, controlHandler controlv1connect.ControlServiceHandler, httpDeps Dependencies) *Server {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	path, handler := controlv1connect.NewControlServiceHandler(controlHandler)
	internalServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, path) {
			t.Fatalf("unexpected rpc path = %q", r.URL.Path)
		}
		handler.ServeHTTP(w, r)
	})}
	errs := make(chan error, 1)
	go func() {
		errs <- internalServer.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = internalServer.Close()
		err := <-errs
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("internal server: %v", err)
		}
	})
	httpDeps.InternalSocketPath = socketPath
	return NewServer(httpDeps)
}

func assertStatus(t *testing.T, server *Server, method string, target string, body *bytes.Buffer, want int) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body == nil {
		reqBody = bytes.NewBuffer(nil)
	} else {
		reqBody = body
	}
	req := httptest.NewRequest(method, target, reqBody)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s status = %d body=%s", method, target, rec.Code, rec.Body.String())
	}
	return rec
}

func writeHTTPConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, dir)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeHTTPPreset(t *testing.T, name string, entries map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "models.ini")
	writeHTTPPresetAtRoot(t, root, name, entries)
	return root
}

func writeHTTPPresets(t *testing.T, presets map[string]map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "models.ini")
	for name, entries := range presets {
		writeHTTPPresetAtRoot(t, root, name, entries)
	}
	return root
}

func writeHTTPPresetAtRoot(t *testing.T, root string, name string, entries map[string]string) {
	t.Helper()
	store := modelpresets.NewStore(root)
	if err := store.Put(context.Background(), modelpresets.Section{Name: name, Values: entries}, true); err != nil {
		t.Fatal(err)
	}
}

type fakeHTTPRuntime struct {
	name    string
	running bool
}

func (f *fakeHTTPRuntime) Status(context.Context) (runtime.Status, error) {
	state := runtime.Stopped
	if f.running {
		state = runtime.Running
	}
	return runtime.Status{State: state, Detail: f.name, CheckedAt: time.Now().UTC(), Processes: []runtime.ProcessStatus{{Name: f.name, State: state, Ready: f.running}}}, nil
}

func (f *fakeHTTPRuntime) Start(context.Context) (runtime.CommandResult, error) {
	f.running = true
	return runtime.CommandResult{Action: "start", ExitCode: 0}, nil
}

func (f *fakeHTTPRuntime) Stop(context.Context) (runtime.CommandResult, error) {
	f.running = false
	return runtime.CommandResult{Action: "stop", ExitCode: 0}, nil
}

func (f *fakeHTTPRuntime) Recover(context.Context) (bool, error) { return false, nil }

type fakeHTTPRouter struct{ models []router.Model }

func newHTTPManager(deps control.Dependencies) *control.Manager {
	if deps.Router == nil {
		deps.Router = &fakeHTTPRouter{}
	}
	if deps.RouterRuntime == nil {
		deps.RouterRuntime = &fakeHTTPRuntime{name: "router"}
	}
	return control.NewManager(deps)
}

func (f *fakeHTTPRouter) List(context.Context) ([]router.Model, error)   { return f.models, nil }
func (f *fakeHTTPRouter) Reload(context.Context) ([]router.Model, error) { return f.models, nil }
func (f *fakeHTTPRouter) Load(_ context.Context, name string) error {
	for _, model := range f.models {
		if model.ID == name {
			return nil
		}
	}
	model := router.Model{ID: name}
	model.Status.Value = "loaded"
	f.models = append(f.models, model)
	return nil
}
func (f *fakeHTTPRouter) Unload(_ context.Context, name string) error {
	for i := range f.models {
		if f.models[i].ID == name {
			f.models = append(f.models[:i], f.models[i+1:]...)
			break
		}
	}
	return nil
}

type fakeModelCatalog struct {
	resolution modelcatalog.Resolution
	rawURL     string
}

type fakeModelDiscoverer struct {
	req     modelcatalog.ListRequest
	machine modelcatalog.MachineProfile
	result  modelcatalog.ListResult
}

func (f *fakeModelDiscoverer) List(_ context.Context, req modelcatalog.ListRequest, machine modelcatalog.MachineProfile) (modelcatalog.ListResult, error) {
	f.req = req
	f.machine = machine
	if f.result.OK {
		return f.result, nil
	}
	return modelcatalog.ListResult{OK: true}, nil
}

type fakeMachineCollector struct {
	snapshot signals.MachineSnapshot
}

type fakeSignalsCollector struct {
	snapshot signals.Snapshot
}

func (f fakeSignalsCollector) Snapshot(context.Context) (signals.Snapshot, error) {
	return f.snapshot, nil
}

func (f fakeMachineCollector) Machine(context.Context) (signals.MachineSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeModelCatalog) Resolve(_ context.Context, rawURL string) (modelcatalog.Resolution, error) {
	f.rawURL = rawURL
	return f.resolution, nil
}

type fakeModelDownloader struct {
	job modeldownload.Job
}

func (f *fakeModelDownloader) Start(context.Context, modeldownload.Request) (modeldownload.Job, error) {
	return f.job, nil
}

func (f *fakeModelDownloader) Get(_ context.Context, _ string) (modeldownload.Job, error) {
	return f.job, nil
}

func (f *fakeModelDownloader) Cancel(_ context.Context, _ string) (modeldownload.Job, error) {
	return f.job, nil
}

func TestHTTPResponsesKeepZeroValuedFields(t *testing.T) {
	discoverer := &fakeModelDiscoverer{result: modelcatalog.ListResult{
		OK:     true,
		Models: []modelcatalog.CatalogModel{{ID: "owner/repo"}},
	}}
	server := newHTTPTestServer(t, rpc.RPCDependencies{
		ModelDiscoverer: discoverer,
		Machine:         fakeMachineCollector{},
	}, Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/models/catalog", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// errors field must be present (not omitted by omitempty) even when empty
	if !strings.Contains(body, `"errors":`) {
		t.Fatalf("missing errors field: %s", body)
	}
	// is_moe must appear as false, not be omitted
	if !strings.Contains(body, `"is_moe":false`) {
		t.Fatalf("missing is_moe:false: %s", body)
	}
}
