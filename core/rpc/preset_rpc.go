package rpc

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"llamarig/core/control"
	"llamarig/core/modelpresets"
	controlv1 "llamarig/core/rpc/gen/v1"
)

func (s *ControlService) autostartSet(ctx context.Context) (map[string]struct{}, int) {
	presets, modelsMax := s.manager.RouterAutostartInfo(ctx)
	set := make(map[string]struct{}, len(presets))
	for _, name := range presets {
		set[name] = struct{}{}
	}
	return set, modelsMax
}

func (s *ControlService) ListPresets(ctx context.Context, _ *controlv1.ListPresetsRequest) (*controlv1.ListPresetsResponse, error) {
	sections, err := s.manager.ListPresets(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	global, err := s.manager.GlobalPreset(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	autostartSet, modelsMax := s.autostartSet(ctx)
	presets := make([]*controlv1.ModelPreset, 0, len(sections))
	for _, section := range sections {
		p := s.presetProto(section)
		_, p.Autostart = autostartSet[section.Name]
		presets = append(presets, p)
	}
	return &controlv1.ListPresetsResponse{Ok: true, Path: s.manager.PresetsRoot(), Global: presetEntries(global), Presets: presets, ModelsMax: int32(modelsMax)}, nil
}

func (s *ControlService) GetPreset(ctx context.Context, req *controlv1.GetPresetRequest) (*controlv1.PresetResponse, error) {
	section, err := s.manager.GetPreset(ctx, req.GetName())
	if err != nil {
		return nil, rpcError(err)
	}
	p := s.presetProto(section)
	autostart, _ := s.autostartSet(ctx)
	_, p.Autostart = autostart[section.Name]
	return &controlv1.PresetResponse{Ok: true, Preset: p}, nil
}

func (s *ControlService) PutPreset(ctx context.Context, req *controlv1.PutPresetRequest) (*controlv1.PresetResponse, error) {
	section, err := s.manager.PutPreset(ctx, sectionFromProto(req.GetPreset()), req.GetCreateOnly())
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.PresetResponse{Ok: true, Preset: s.presetProto(section)}, nil
}

func (s *ControlService) DeletePreset(ctx context.Context, req *controlv1.DeletePresetRequest) (*controlv1.MutationResponse, error) {
	if err := s.manager.DeletePreset(ctx, req.GetName()); err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.MutationResponse{Ok: true}, nil
}

func (s *ControlService) CleanupPreset(ctx context.Context, req *controlv1.CleanupPresetRequest) (*controlv1.MutationResponse, error) {
	if req == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "request is required"))
	}
	result, err := s.manager.CleanupPreset(ctx, req.GetName())
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.MutationResponse{Ok: true, Result: operationResultProto(result)}, nil
}

func (s *ControlService) SetPresetAutostart(ctx context.Context, req *controlv1.PresetAutostartRequest) (*controlv1.MutationResponse, error) {
	if req == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "request is required"))
	}
	if req.GetName() == "" {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "preset name is required"))
	}
	_, err := s.manager.SetPresetAutostart(ctx, req.GetName(), req.GetEnabled())
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.MutationResponse{Ok: true}, nil
}

func (s *ControlService) presetProto(section modelpresets.Section) *controlv1.ModelPreset {
	status := s.presetSources.status(section)
	return &controlv1.ModelPreset{Name: section.Name, Entries: presetEntries(section), SourceStatus: status.State, SourceError: status.Error}
}

// presetEntries renders a section's key/value map in a stable order: model
// sources first, then the remaining keys alphabetically.
func presetEntries(section modelpresets.Section) []*controlv1.PresetEntry {
	rank := func(key string) string {
		if key == "model" || key == "models-dir" {
			return "0" + key // model sources sort first
		}
		return "1" + key
	}
	keys := slices.SortedFunc(maps.Keys(section.Values), func(a, b string) int { return cmp.Compare(rank(a), rank(b)) })
	entries := make([]*controlv1.PresetEntry, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, &controlv1.PresetEntry{Key: key, Value: section.Values[key]})
	}
	return entries
}

func sectionFromProto(preset *controlv1.ModelPreset) modelpresets.Section {
	if preset == nil {
		return modelpresets.Section{}
	}
	values := make(map[string]string, len(preset.GetEntries()))
	for _, entry := range preset.GetEntries() {
		values[entry.GetKey()] = entry.GetValue()
	}
	return modelpresets.Section{Name: preset.GetName(), Values: values}
}
