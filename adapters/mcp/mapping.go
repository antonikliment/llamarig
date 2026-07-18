package mcp

import (
	controlv1 "llamarig/core/rpc/gen/v1"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mappedToolOutput[T any, O any](out T, err error, mapFn func(T) O) (*mcp.CallToolResult, O, error) {
	var zero O
	if err != nil {
		return nil, zero, err
	}
	return nil, mapFn(out), nil
}

func presetEntriesMap(entries []*controlv1.PresetEntry) map[string]string {
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		out[entry.GetKey()] = entry.GetValue()
	}
	return out
}

func presetMap(preset *controlv1.ModelPreset) map[string]any {
	if preset == nil {
		return nil
	}
	return map[string]any{"name": preset.GetName(), "entries": presetEntriesMap(preset.GetEntries()), "source_status": preset.GetSourceStatus(), "source_error": preset.GetSourceError()}
}

func presetMaps(presets []*controlv1.ModelPreset) []map[string]any {
	out := make([]map[string]any, 0, len(presets))
	for _, preset := range presets {
		out = append(out, presetMap(preset))
	}
	return out
}

func presetEntriesProto(entries map[string]string) []*controlv1.PresetEntry {
	out := make([]*controlv1.PresetEntry, 0, len(entries))
	for key, value := range entries {
		out = append(out, &controlv1.PresetEntry{Key: key, Value: value})
	}
	return out
}
