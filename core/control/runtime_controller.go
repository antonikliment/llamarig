package control

import (
	"context"
	"fmt"
	"time"

	"llamarig/core/modelpresets"
	"llamarig/core/runtime"
)

func (m *Manager) Status(ctx context.Context) (RuntimeStatus, error) {
	return m.routerStatus(ctx)
}

func (m *Manager) StartOperation(ctx context.Context, server string) (OperationResult, error) {
	return m.routerOperation(ctx, "start", server)
}

func (m *Manager) StopOperation(ctx context.Context, server string) (OperationResult, error) {
	if server == "" {
		return withMutationResult(m, ctx, "stop_router", OperationResult{}, func() (OperationResult, error) {
			if m.routerRuntime == nil {
				return OperationResult{}, Errorf(ErrorInvalidInput, "router runtime is not configured")
			}
			result, err := m.routerRuntime.Stop(ctx)
			return operationFromRuntimeResult("stop", "all", result, err), err
		})
	}
	return m.routerOperation(ctx, "stop", server)
}

func (m *Manager) RestartOperation(ctx context.Context, server string) (OperationResult, error) {
	return m.routerOperation(ctx, "restart", server)
}

func (m *Manager) StartAutostartPresets(ctx context.Context) []error {
	errs := make([]error, 0)
	for _, name := range m.routerConfigSnapshot(ctx).AutostartPresets {
		if _, err := m.StartOperation(ctx, name); err != nil {
			errs = append(errs, fmt.Errorf("autostart Preset %q: %w", name, err))
		}
	}
	return errs
}

func (m *Manager) RecoverRouterRuntime(ctx context.Context) error {
	if m.routerRuntime == nil {
		return Errorf(ErrorInvalidInput, "router runtime is not configured")
	}
	_, err := m.routerRuntime.Recover(ctx)
	return err
}

func (m *Manager) routerOperation(ctx context.Context, action, requested string) (OperationResult, error) {
	return withMutationResult(m, ctx, action, OperationResult{}, func() (OperationResult, error) {
		name, err := m.resolveStartPreset(ctx, requested)
		if err != nil {
			return OperationResult{}, err
		}
		if action == "start" || action == "restart" {
			if err := m.requirePresetSource(ctx, name); err != nil {
				return OperationResult{}, err
			}
		}
		if err := m.ensureRouter(ctx); err != nil {
			return OperationResult{}, err
		}
		if err := m.applyRouterAction(ctx, action, name); err != nil {
			return OperationResult{}, mapRuntimeError(err, "router "+action+" failed")
		}
		return OperationResult{Action: action, Target: name, Status: "succeeded", Message: action + " completed"}, nil
	})
}

func (m *Manager) requirePresetSource(ctx context.Context, name string) error {
	preset, err := m.GetPreset(ctx, name)
	if err != nil {
		return err
	}
	status := modelpresets.InspectSource(preset)
	if status.State != modelpresets.SourceReady {
		return Errorf(ErrorConflict, "Preset %q is unavailable: %s", name, status.Error)
	}
	return nil
}

func (m *Manager) applyRouterAction(ctx context.Context, action, name string) error {
	if m.router == nil {
		return Errorf(ErrorInvalidInput, "router client is not configured")
	}
	switch action {
	case "start":
		return m.router.Load(ctx, name)
	case "stop":
		return m.router.Unload(ctx, name)
	case "restart":
		// Best-effort unload: a restart that targets a model which is not
		// currently loaded (e.g. right after a preset edit) should still load it.
		_ = m.router.Unload(ctx, name)
		return m.router.Load(ctx, name)
	default:
		return Errorf(ErrorInvalidInput, "unknown router action %q", action)
	}
}

func (m *Manager) routerStatus(ctx context.Context) (RuntimeStatus, error) {
	if m.router == nil {
		return m.routerUnavailableStatus(ctx)
	}
	models, err := m.router.List(ctx)
	if err != nil {
		return m.routerUnavailableStatus(ctx)
	}
	status := RuntimeStatus{State: string(runtime.Stopped), Detail: "no presets loaded", CheckedAt: time.Now().UTC()}
	for _, model := range models {
		state := runtime.Stopped
		switch model.Status.Value {
		case "loaded", "sleeping":
			state = runtime.Running
		case "loading", "downloading":
			state = runtime.Starting
		}
		if state == runtime.Stopped {
			continue
		}
		status.Presets = append(status.Presets, RuntimePreset{Name: model.ID, State: string(state), Ready: state == runtime.Running})
		if state == runtime.Starting || status.State == string(runtime.Stopped) {
			status.State = string(state)
		}
	}
	if len(status.Presets) > 0 {
		status.Detail = fmt.Sprintf("%d preset(s) active", len(status.Presets))
	}
	return status, nil
}

func (m *Manager) routerUnavailableStatus(ctx context.Context) (RuntimeStatus, error) {
	if m.routerRuntime == nil {
		return RuntimeStatus{State: string(runtime.Stopped), Detail: "router runtime is not configured", CheckedAt: time.Now().UTC()}, nil
	}
	process, err := m.routerRuntime.Status(ctx)
	if err != nil {
		return RuntimeStatus{}, mapRuntimeError(err, "router runtime status failed")
	}
	checkedAt := process.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	detail := "router process " + string(process.State) + "; model list unavailable"
	if process.State == runtime.Stopped {
		detail = "router stopped"
	}
	return RuntimeStatus{State: string(process.State), Detail: detail, CheckedAt: checkedAt}, nil
}

func (m *Manager) ensureRouter(ctx context.Context) error {
	if m.routerRuntime == nil {
		return Errorf(ErrorInvalidInput, "router runtime is not configured")
	}
	status, err := m.routerRuntime.Status(ctx)
	if err != nil {
		return mapRuntimeError(err, "router runtime status failed")
	}
	if status.State != runtime.Stopped {
		return nil
	}
	if _, err := m.routerRuntime.Start(ctx); err != nil {
		return mapRuntimeError(err, "router runtime start failed")
	}
	return nil
}

func (m *Manager) resolveStartPreset(ctx context.Context, requested string) (string, error) {
	name := requested
	if defaultPreset := m.routerConfigSnapshot(ctx).DefaultPreset; requested == "" && defaultPreset != "" {
		name = defaultPreset
	}
	if name != "" {
		if _, err := m.GetPreset(ctx, name); err != nil {
			return "", err
		}
		return name, nil
	}
	presets, err := m.ListPresets(ctx)
	if err != nil {
		return "", err
	}
	if len(presets) == 1 {
		return presets[0].Name, nil
	}
	names := make([]string, 0, len(presets))
	for _, preset := range presets {
		names = append(names, preset.Name)
	}
	return "", Errorf(ErrorInvalidInput, "server is required; available servers: %v", names)
}
