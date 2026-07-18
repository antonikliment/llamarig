package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestGatewayRESTFlow drives the public_http REST gateway end-to-end against a
// real in-process control service: preset CRUD, a stub runtime lifecycle, and
// the read-only telemetry endpoints.
func TestGatewayRESTFlow(t *testing.T) {
	gw := startGateway(t)
	name := "gw-preset"
	modelPath := filepath.Join(t.TempDir(), name+".gguf")
	if err := os.WriteFile(modelPath, []byte("gguf"), 0o600); err != nil {
		t.Fatal(err)
	}

	gatewayOK(t, gw, http.MethodGet, "/health", "")
	gatewayOK(t, gw, http.MethodGet, "/api/info", "")
	gatewayOK(t, gw, http.MethodGet, "/api/signals", "")
	gatewayOK(t, gw, http.MethodGet, "/api/events", "")
	gatewayOK(t, gw, http.MethodGet, "/api/models/local", "")

	created := gatewayJSON(t, gw, http.MethodPost, "/api/presets", fmt.Sprintf(`{"name":%q,"entries":[{"key":"model","value":%q}]}`, name, modelPath))
	if created["ok"] != true {
		t.Fatalf("create preset: %#v", created)
	}

	list := gatewayJSON(t, gw, http.MethodGet, "/api/presets", "")
	if !gatewayListHasPreset(list, name) {
		t.Fatalf("created preset missing from list: %#v", list)
	}
	gatewayOK(t, gw, http.MethodGet, "/api/presets/"+name, "")

	gatewayOK(t, gw, http.MethodPost, "/api/runtime/start?preset="+name, "")
	gatewayWaitRunning(t, gw, name)
	for _, target := range []string{"/api/runtime/resources", "/api/config.yaml"} {
		if rec := gatewayDo(t, gw, http.MethodGet, target, ""); rec.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
	gatewayOK(t, gw, http.MethodPost, "/api/runtime/stop?preset="+name, "")
	gatewayOK(t, gw, http.MethodDelete, "/api/presets/"+name, "")
}

// TestGatewayMCPSession drives the MCP adapter over the gateway's /mcp HTTP
// mount end-to-end against the real control service.
func TestGatewayMCPSession(t *testing.T) {
	server := httptest.NewServer(startGateway(t))
	t.Cleanup(server.Close)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "v0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect mcp session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	tools := map[string]bool{}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		tools[tool.Name] = true
	}
	for _, want := range []string{"llama_info", "llama_status", "presets_list", "preset_put"} {
		if !tools[want] {
			t.Fatalf("missing mcp tool %q in %#v", want, tools)
		}
	}

	name := "mcp-gw"
	requireMCPTool(t, session, "preset_put", map[string]any{"name": name, "entries": map[string]any{"model": "/models/mcp-gw.gguf"}})
	listed := requireMCPTool(t, session, "presets_list", map[string]any{})
	if structured, ok := listed.StructuredContent.(map[string]any); !ok || structured["presets"] == nil {
		t.Fatalf("presets_list output = %#v", listed.StructuredContent)
	}
	requireMCPTool(t, session, "llama_status", map[string]any{})

	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "presets://" + name})
	if err != nil {
		t.Fatalf("read preset resource: %v", err)
	}
	if resource == nil || len(resource.Contents) == 0 {
		t.Fatalf("read preset resource: empty or nil resource")
	}
	if got := resource.Contents[0].Text; !strings.Contains(got, "mcp-gw.gguf") {
		t.Fatalf("resource text = %q", got)
	}
}

func requireMCPTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("%s returned tool error: %#v", name, result)
	}
	return result
}

func gatewayDo(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gatewayOK(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := gatewayDo(t, h, method, target, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s -> %d: %s", method, target, rec.Code, rec.Body.String())
	}
	return rec
}

func gatewayJSON(t *testing.T, h http.Handler, method, target, body string) map[string]any {
	t.Helper()
	rec := gatewayOK(t, h, method, target, body)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s decode: %v body=%s", method, target, err, rec.Body.String())
		}
	}
	return out
}

func gatewayListHasPreset(list map[string]any, name string) bool {
	presets, _ := list["presets"].([]any)
	for _, raw := range presets {
		if preset, ok := raw.(map[string]any); ok && preset["name"] == name {
			return true
		}
	}
	return false
}

func gatewayWaitRunning(t *testing.T, h http.Handler, name string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		status := gatewayJSON(t, h, http.MethodGet, "/api/runtime/status", "")
		presets, _ := status["presets"].([]any)
		for _, raw := range presets {
			preset, ok := raw.(map[string]any)
			if ok && preset["name"] == name && preset["state"] == "running" && preset["ready"] == true {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("runtime %q did not reach running/ready", name)
}
