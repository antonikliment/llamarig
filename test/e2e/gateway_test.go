package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

// TestGatewayRPCFlow drives the public Connect RPC gateway end-to-end against a
// real in-process control service: preset CRUD, a stub runtime lifecycle, and
// the read-only telemetry endpoints.
func TestGatewayRPCFlow(t *testing.T) {
	server := httptest.NewServer(startGateway(t))
	t.Cleanup(server.Close)
	client := controlv1connect.NewControlServiceClient(server.Client(), server.URL)
	ctx := context.Background()
	name := "gw-preset"
	modelPath := filepath.Join(t.TempDir(), name+".gguf")
	if err := os.WriteFile(modelPath, []byte("gguf"), 0o600); err != nil {
		t.Fatal(err)
	}

	assertGatewayReads(t, client)

	created, err := client.PutPreset(ctx, &controlv1.PutPresetRequest{
		Preset:     &controlv1.ModelPreset{Name: name, Entries: []*controlv1.PresetEntry{{Key: "model", Value: modelPath}}},
		CreateOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.GetOk() {
		t.Fatalf("create preset: %#v", created)
	}

	list, err := client.ListPresets(ctx, &controlv1.ListPresetsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcListHasPreset(list, name) {
		t.Fatalf("created preset missing from list: %#v", list)
	}
	if _, err := client.GetPreset(ctx, &controlv1.GetPresetRequest{Name: name}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.StartRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name}); err != nil {
		t.Fatal(err)
	}
	rpcWaitRunning(t, client, name)
	assertRemovedRoutes(t, server)
	if _, err := client.StopRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DeletePreset(ctx, &controlv1.DeletePresetRequest{Name: name}); err != nil {
		t.Fatal(err)
	}
}

func assertGatewayReads(t *testing.T, client controlv1connect.ControlServiceClient) {
	t.Helper()
	ctx := context.Background()
	checks := []error{}
	_, err := client.Health(ctx, &controlv1.HealthRequest{})
	checks = append(checks, err)
	_, err = client.GetInfo(ctx, &controlv1.GetInfoRequest{})
	checks = append(checks, err)
	_, err = client.GetSignals(ctx, &controlv1.GetSignalsRequest{})
	checks = append(checks, err)
	_, err = client.ListEvents(ctx, &controlv1.ListEventsRequest{})
	checks = append(checks, err)
	_, err = client.ListLocalModels(ctx, &controlv1.ListLocalModelsRequest{})
	checks = append(checks, err)
	for _, err := range checks {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertRemovedRoutes(t *testing.T, server *httptest.Server) {
	t.Helper()
	for _, target := range []string{"/api/runtime/resources", "/api/config.yaml"} {
		response, err := server.Client().Get(server.URL + target)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status=%d", target, response.StatusCode)
		}
	}
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

func rpcListHasPreset(list *controlv1.ListPresetsResponse, name string) bool {
	for _, preset := range list.GetPresets() {
		if preset.GetName() == name {
			return true
		}
	}
	return false
}

func rpcWaitRunning(t *testing.T, client controlv1connect.ControlServiceClient, name string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		status, err := client.GetRuntimeStatus(context.Background(), &controlv1.GetRuntimeStatusRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for _, preset := range status.GetStatus().GetPresets() {
			if preset.GetName() == name && preset.GetState() == "running" && preset.GetReady() {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("runtime %q did not reach running/ready", name)
}
