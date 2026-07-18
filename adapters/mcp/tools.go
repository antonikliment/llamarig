package mcp

import (
	"context"

	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addTools(server *mcp.Server, client controlv1connect.ControlServiceClient) {
	mcp.AddTool(server, &mcp.Tool{Name: "llama_info", Description: "Return llama runtime info"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, *controlv1.RuntimeInfo, error) {
		out, err := client.GetInfo(ctx, &controlv1.GetInfoRequest{})
		return mappedToolOutput(out, err, func(out *controlv1.GetInfoResponse) *controlv1.RuntimeInfo { return out.GetInfo() })
	})
	mcp.AddTool(server, &mcp.Tool{Name: "llama_status", Description: "Return llama runtime status"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, *controlv1.RuntimeStatus, error) {
		out, err := client.GetRuntimeStatus(ctx, &controlv1.GetRuntimeStatusRequest{})
		return mappedToolOutput(out, err, func(out *controlv1.GetRuntimeStatusResponse) *controlv1.RuntimeStatus { return out.GetStatus() })
	})
	addRuntimeTool(server, "llama_start", "Load a model preset", client.StartRuntime)
	addRuntimeTool(server, "llama_stop", "Unload a model preset or stop the router", client.StopRuntime)
	addRuntimeTool(server, "llama_restart", "Reload a model preset", client.RestartRuntime)
	mcp.AddTool(server, &mcp.Tool{Name: "presets_list", Description: "List models.ini presets"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, map[string]any, error) {
		out, err := client.ListPresets(ctx, &controlv1.ListPresetsRequest{})
		return mappedToolOutput(out, err, func(out *controlv1.ListPresetsResponse) map[string]any {
			return map[string]any{"path": out.GetPath(), "global": presetEntriesMap(out.GetGlobal()), "presets": presetMaps(out.GetPresets())}
		})
	})
	mcp.AddTool(server, &mcp.Tool{Name: "preset_get", Description: "Read one models.ini preset"}, func(ctx context.Context, _ *mcp.CallToolRequest, input presetNameInput) (*mcp.CallToolResult, map[string]any, error) {
		out, err := client.GetPreset(ctx, &controlv1.GetPresetRequest{Name: input.Name})
		return mappedToolOutput(out, err, func(out *controlv1.PresetResponse) map[string]any { return presetMap(out.GetPreset()) })
	})
	mcp.AddTool(server, &mcp.Tool{Name: "preset_put", Description: "Create or replace a models.ini preset"}, func(ctx context.Context, _ *mcp.CallToolRequest, input presetPutInput) (*mcp.CallToolResult, map[string]any, error) {
		preset := &controlv1.ModelPreset{Name: input.Name, Entries: presetEntriesProto(input.Entries)}
		out, err := client.PutPreset(ctx, &controlv1.PutPresetRequest{Preset: preset, CreateOnly: input.CreateOnly})
		return mappedToolOutput(out, err, func(out *controlv1.PresetResponse) map[string]any { return presetMap(out.GetPreset()) })
	})
	mcp.AddTool(server, &mcp.Tool{Name: "preset_delete", Description: "Delete a models.ini preset"}, func(ctx context.Context, _ *mcp.CallToolRequest, input presetNameInput) (*mcp.CallToolResult, any, error) {
		out, err := client.DeletePreset(ctx, &controlv1.DeletePresetRequest{Name: input.Name})
		return mappedToolOutput(out, err, func(out *controlv1.MutationResponse) any { return map[string]any{"ok": out.GetOk()} })
	})
}

func addRuntimeTool(server *mcp.Server, name, desc string, call func(context.Context, *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: desc}, func(ctx context.Context, _ *mcp.CallToolRequest, input runtimeInput) (*mcp.CallToolResult, *controlv1.CommandResult, error) {
		out, err := call(ctx, &controlv1.RuntimeTargetRequest{Target: input.Preset})
		return mappedToolOutput(out, err, func(out *controlv1.CommandResponse) *controlv1.CommandResult { return out.GetResult() })
	})
}
