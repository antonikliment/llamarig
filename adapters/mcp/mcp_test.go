package mcp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"llamarig/core/control"
	"llamarig/core/modelpresets"
	"llamarig/core/router"
	"llamarig/core/rpc"
	"llamarig/core/rpc/gen/v1/controlv1connect"
	"llamarig/core/runtime"
	"llamarig/internal/buildinfo"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPToolsAndResourceFlow(t *testing.T) {
	manager := control.NewManager(control.Dependencies{
		Presets:       modelpresets.NewStore(writeMCPProfile(t, "qwen")),
		Router:        &fakeMCPRouter{},
		RouterRuntime: &fakeMCPRuntime{name: "router"},
	})
	session := connectMCPTestSession(t, manager)
	defer closeMCPSession(t, session)

	assertMCPTools(t, session)

	info, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "llama_info", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("llama_info: %v", err)
	}
	if info.IsError {
		t.Fatalf("llama_info returned tool error: %#v", info)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "preset_get", Arguments: map[string]any{"name": "qwen"}})
	if err != nil {
		t.Fatalf("preset_get: %v", err)
	}
	if result.IsError {
		t.Fatalf("preset_get returned tool error: %#v", result)
	}
	if result.StructuredContent == nil {
		t.Fatalf("preset_get returned no structured RPC output: %#v", result)
	}
	profiles, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "presets_list", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("presets_list: %v", err)
	}
	if profiles.IsError {
		t.Fatalf("presets_list returned tool error: %#v", profiles)
	}
	structured, ok := profiles.StructuredContent.(map[string]any)
	if !ok || structured["presets"] == nil {
		t.Fatalf("presets_list output = %#v", profiles.StructuredContent)
	}
	resource, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "presets://qwen"})
	if err != nil {
		t.Fatalf("read preset resource: %v", err)
	}
	if got := resource.Contents[0].Text; !strings.Contains(got, "models-dir") {
		t.Fatalf("resource text = %q", got)
	}
}

func TestMCPServerVersion(t *testing.T) {
	manager := control.NewManager(control.Dependencies{})
	session := connectMCPTestSession(t, manager)
	defer closeMCPSession(t, session)
	if got := session.InitializeResult().ServerInfo.Version; got != buildinfo.Version {
		t.Fatalf("MCP server version = %q, want %q", got, buildinfo.Version)
	}
}

func TestParsePresetResourceURIUnescapesName(t *testing.T) {
	name, ok := parsePresetResourceURI("presets://My%20Preset")
	if !ok || name != "My Preset" {
		t.Fatalf("parsePresetResourceURI() = %q, %v", name, ok)
	}
	if _, ok := parsePresetResourceURI("presets://bad%zz"); ok {
		t.Fatal("parsePresetResourceURI() accepted invalid escape")
	}
}

func connectMCPTestSession(t *testing.T, manager *control.Manager) *mcp.ClientSession {
	t.Helper()
	server := NewServer(Dependencies{ControlClient: newInProcessControlClient(manager), ServiceName: "test-service"})
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.1.0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	return session
}

func closeMCPSession(t *testing.T, session *mcp.ClientSession) {
	t.Helper()
	if err := session.Close(); err != nil {
		t.Fatalf("session close: %v", err)
	}
}

func assertMCPTools(t *testing.T, session *mcp.ClientSession) {
	t.Helper()
	tools := map[string]bool{}
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		tools[tool.Name] = true
	}
	for _, name := range []string{"llama_info", "llama_status", "llama_start", "llama_stop", "llama_restart", "presets_list", "preset_get", "preset_put", "preset_delete"} {
		if !tools[name] {
			t.Fatalf("missing tool %q in %#v", name, tools)
		}
	}
}

func writeMCPProfile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "models.ini")
	store := modelpresets.NewStore(path)
	if err := store.Put(context.Background(), modelpresets.Section{Name: name, Values: map[string]string{"models-dir": "./models"}}, true); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMCPStartToolError(t *testing.T) {
	manager := control.NewManager(control.Dependencies{})
	server := NewServer(Dependencies{ControlClient: newInProcessControlClient(manager), ServiceName: "test-service"})
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.1.0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	if _, err := server.Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Fatalf("session close: %v", err)
		}
	}()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "llama_start", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("llama_start protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error: %#v", result)
	}
}

func newInProcessControlClient(manager *control.Manager) controlv1connect.ControlServiceClient {
	path, handler := controlv1connect.NewControlServiceHandler(rpc.NewControlService(rpc.RPCDependencies{Manager: manager, ServiceName: "test-service"}))
	client := &http.Client{Transport: inProcessTransport{path: path, handler: handler}}
	return controlv1connect.NewControlServiceClient(client, "http://in-process")
}

type inProcessTransport struct {
	path    string
	handler http.Handler
}

func (t inProcessTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	if resp.Body == nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
	}
	return resp, nil
}

type fakeMCPRuntime struct {
	name    string
	running bool
}

func (f *fakeMCPRuntime) Status(context.Context) (runtime.Status, error) {
	state := runtime.Stopped
	if f.running {
		state = runtime.Running
	}
	return runtime.Status{State: state, Detail: f.name, CheckedAt: time.Now().UTC()}, nil
}

func (f *fakeMCPRuntime) Start(context.Context) (runtime.CommandResult, error) {
	f.running = true
	return runtime.CommandResult{Action: "start", ExitCode: 0}, nil
}

func (f *fakeMCPRuntime) Stop(context.Context) (runtime.CommandResult, error) {
	f.running = false
	return runtime.CommandResult{Action: "stop", ExitCode: 0}, nil
}

func (f *fakeMCPRuntime) Recover(context.Context) (bool, error) { return false, nil }

type fakeMCPRouter struct{ models []router.Model }

func (f *fakeMCPRouter) List(context.Context) ([]router.Model, error)   { return f.models, nil }
func (f *fakeMCPRouter) Reload(context.Context) ([]router.Model, error) { return f.models, nil }
func (f *fakeMCPRouter) Load(_ context.Context, name string) error {
	model := router.Model{ID: name}
	model.Status.Value = "loaded"
	f.models = append(f.models, model)
	return nil
}
func (f *fakeMCPRouter) Unload(_ context.Context, name string) error {
	for i := range f.models {
		if f.models[i].ID == name {
			f.models = append(f.models[:i], f.models[i+1:]...)
			break
		}
	}
	return nil
}
