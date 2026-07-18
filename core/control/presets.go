package control

import (
	"context"
	"errors"

	"llamarig/core/modelpresets"
	"llamarig/core/runtime"
)

func (m *Manager) requirePresets() error {
	if m.presets == nil {
		return Errorf(ErrorInvalidInput, "preset store is not configured")
	}
	return nil
}

func (m *Manager) PresetsRoot() string {
	if m.presets == nil {
		return ""
	}
	return m.presets.Root()
}

func (m *Manager) ListPresets(ctx context.Context) ([]modelpresets.Section, error) {
	if err := m.requirePresets(); err != nil {
		return nil, err
	}
	sections, err := m.presets.List(ctx)
	if err != nil {
		return nil, mapServerConfigError(err)
	}
	return sections, nil
}

func (m *Manager) GlobalPreset(ctx context.Context) (modelpresets.Section, error) {
	if err := m.requirePresets(); err != nil {
		return modelpresets.Section{}, err
	}
	section, err := m.presets.Global(ctx)
	if err != nil {
		return modelpresets.Section{}, mapServerConfigError(err)
	}
	return section, nil
}

func (m *Manager) GetPreset(ctx context.Context, name string) (modelpresets.Section, error) {
	if err := m.requirePresets(); err != nil {
		return modelpresets.Section{}, err
	}
	section, err := m.presets.Get(ctx, name)
	if err != nil {
		return modelpresets.Section{}, mapServerConfigError(err)
	}
	return section, nil
}

func (m *Manager) PutPreset(ctx context.Context, section modelpresets.Section, createOnly bool) (modelpresets.Section, error) {
	return withMutationResult(m, ctx, "put_preset", modelpresets.Section{}, func() (modelpresets.Section, error) {
		if err := m.requirePresets(); err != nil {
			return modelpresets.Section{}, err
		}
		previousRouterState := routerState{}
		if createOnly {
			previousRouterState = m.captureRouterState(ctx)
		}
		if err := m.presets.Put(ctx, section, createOnly); err != nil {
			return modelpresets.Section{}, mapServerConfigError(err)
		}
		saved, err := m.presets.Get(ctx, section.Name)
		if err != nil {
			return modelpresets.Section{}, mapServerConfigError(err)
		}
		if _, err := m.refreshRouterSources(ctx, "Preset "+section.Name+" changed"); err != nil {
			if createOnly {
				return modelpresets.Section{}, m.rollbackPresetCreation(ctx, section.Name, previousRouterState, err)
			}
			return saved, CoreError(ErrorRuntime, "Preset saved; Router refresh failed", err)
		}
		return saved, nil
	})
}

func (m *Manager) rollbackPresetCreation(ctx context.Context, name string, previous routerState, refreshErr error) error {
	if err := m.presets.Delete(ctx, name); err != nil {
		return CoreError(ErrorRuntime, "Preset creation failed and could not be rolled back: "+err.Error(), errors.Join(refreshErr, err))
	}
	if previous.running {
		if _, err := m.restartRouterSources(context.WithoutCancel(ctx), previous.active, "Preset "+name+" creation reverted"); err != nil {
			return CoreError(ErrorRuntime, "Preset creation reverted; Router recovery failed: "+err.Error(), errors.Join(refreshErr, err))
		}
	}
	return CoreError(ErrorRuntime, "Router refresh failed with the new preset; creation reverted", refreshErr)
}

func (m *Manager) DeletePreset(ctx context.Context, name string) error {
	_, err := withMutationResult(m, ctx, "delete_preset", struct{}{}, func() (struct{}, error) {
		if err := m.requirePresets(); err != nil {
			return struct{}{}, err
		}
		deletable, err := m.presetDeletable(ctx, name)
		if err != nil {
			return struct{}{}, err
		}
		if !deletable {
			return struct{}{}, nil
		}
		if err := m.presets.Delete(ctx, name); err != nil {
			return struct{}{}, mapServerConfigError(err)
		}
		if _, err := m.refreshRouterSources(ctx, "Preset "+name+" deleted"); err != nil {
			return struct{}{}, CoreError(ErrorRuntime, "Preset deleted; Router refresh failed", err)
		}
		return struct{}{}, nil
	})
	return err
}

func (m *Manager) presetDeletable(ctx context.Context, name string) (bool, error) {
	exists, err := m.presetExists(ctx, name)
	if err != nil || !exists {
		return false, err
	}
	if reference := m.presetConfigReference(ctx, name); reference != "" {
		return false, Errorf(ErrorConflict, "Preset %q is configured in %s", name, reference)
	}
	running, err := m.modelRunning(ctx, name)
	if err != nil {
		return false, err
	}
	if running {
		return false, Errorf(ErrorConflict, "preset %q is loaded", name)
	}
	return true, nil
}

func (m *Manager) presetExists(ctx context.Context, name string) (bool, error) {
	if _, err := m.presets.Get(ctx, name); err != nil {
		if errors.Is(err, modelpresets.ErrNotFound) {
			return false, nil
		}
		return false, mapServerConfigError(err)
	}
	return true, nil
}

func (m *Manager) presetConfigReference(ctx context.Context, name string) string {
	cfg := m.routerConfigSnapshot(ctx)
	if name == cfg.DefaultPreset {
		return "router.default_preset"
	}
	for _, autostart := range cfg.AutostartPresets {
		if name == autostart {
			return "router.autostart_presets"
		}
	}
	return ""
}

func (m *Manager) CleanupPreset(ctx context.Context, name string) (OperationResult, error) {
	return withMutationResult(m, ctx, "cleanup_preset", OperationResult{}, func() (OperationResult, error) {
		return m.cleanupPreset(ctx, name)
	})
}

func (m *Manager) cleanupPreset(ctx context.Context, name string) (OperationResult, error) {
	if err := m.requirePresets(); err != nil {
		return OperationResult{}, err
	}
	if name == "" {
		return OperationResult{}, Errorf(ErrorInvalidInput, "Preset name is required")
	}
	exists, err := m.presetExists(ctx, name)
	if err != nil {
		return OperationResult{}, err
	}
	if exists {
		if err := m.requireUnavailablePreset(ctx, name); err != nil {
			return OperationResult{}, err
		}
	}
	if err := m.removePresetReferences(ctx, name); err != nil {
		return OperationResult{}, err
	}
	if !exists {
		return cleanupResult(name), nil
	}
	if err := m.presets.Delete(ctx, name); err != nil {
		return OperationResult{}, mapServerConfigError(err)
	}
	if _, err := m.refreshRouterSources(ctx, "Unavailable Preset "+name+" cleaned up"); err != nil {
		return OperationResult{}, CoreError(ErrorRuntime, "Preset removed; Router refresh failed", err)
	}
	return cleanupResult(name), nil
}

func cleanupResult(name string) OperationResult {
	return OperationResult{Action: "cleanup", Target: name, Status: "succeeded", Message: "unavailable Preset removed"}
}

func (m *Manager) requireUnavailablePreset(ctx context.Context, name string) error {
	preset, err := m.GetPreset(ctx, name)
	if err != nil {
		return err
	}
	if modelpresets.InspectSource(preset).State == modelpresets.SourceReady {
		return Errorf(ErrorConflict, "Preset %q is available and cannot be cleaned up", name)
	}
	running, err := m.modelRunning(ctx, name)
	if err != nil {
		return err
	}
	if running {
		return Errorf(ErrorConflict, "preset %q is loaded", name)
	}
	return nil
}

func (m *Manager) removePresetReferences(ctx context.Context, names ...string) error {
	if m.config == nil {
		for _, name := range names {
			if m.presetConfigReference(ctx, name) != "" {
				return Errorf(ErrorInvalidInput, "config store is not configured")
			}
		}
		return nil
	}
	if err := m.config.RemoveRouterPresetReferences(ctx, names...); err != nil {
		return mapConfigStoreError(err)
	}
	return nil
}

func (m *Manager) modelRunning(ctx context.Context, name string) (bool, error) {
	if m.routerRuntime == nil || m.router == nil {
		return false, nil
	}
	status, err := m.routerRuntime.Status(ctx)
	if err != nil {
		return false, mapRuntimeError(err, "router runtime status failed")
	}
	if status.State == runtime.Stopped {
		return false, nil
	}
	models, err := m.router.List(ctx)
	if err != nil {
		return false, mapRuntimeError(err, "router status failed")
	}
	for _, model := range models {
		if model.ID == name && (model.Status.Value == "loaded" || model.Status.Value == "sleeping" || model.Status.Value == "loading" || model.Status.Value == "downloading") {
			return true, nil
		}
	}
	return false, nil
}
