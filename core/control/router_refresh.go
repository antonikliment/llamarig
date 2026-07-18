package control

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"llamarig/core/router"
	"llamarig/core/runtime"
)

type routerState struct {
	running bool
	active  []string
}

func (m *Manager) RefreshRouterSources(ctx context.Context, reason string) (OperationResult, error) {
	return withMutationResult(m, ctx, "refresh_router_sources", OperationResult{}, func() (OperationResult, error) {
		return m.refreshRouterSources(ctx, reason)
	})
}

func (m *Manager) refreshRouterSources(ctx context.Context, reason string) (OperationResult, error) {
	ctx = context.WithoutCancel(ctx)
	result := OperationResult{Action: "refresh_router_sources", Target: "router", Status: "succeeded", Message: "router sources refreshed: " + reason}
	if m.routerRuntime == nil || m.router == nil {
		result.Status, result.Message = "skipped", "router refresh skipped: runtime is not configured"
		return result, nil
	}
	status, err := m.routerRuntime.Status(ctx)
	if err != nil {
		return result, mapRuntimeError(err, "router runtime status failed")
	}
	if status.State == runtime.Stopped {
		result.Status, result.Message = "skipped", "router stopped; fresh sources will be read on next start"
		return result, nil
	}
	models, err := m.router.List(ctx)
	if err != nil {
		return m.restartRouterSources(ctx, nil, reason)
	}
	active := activeModelNames(models)
	refreshed, err := m.router.Reload(ctx)
	if err != nil {
		return m.restartRouterSources(ctx, active, reason)
	}
	if err := m.restoreActiveModels(ctx, active, refreshed); err != nil {
		return result, CoreError(ErrorRuntime, "router sources refreshed; active Preset restore failed", err)
	}
	return result, nil
}

func (m *Manager) captureRouterState(ctx context.Context) (state routerState) {
	if m.routerRuntime == nil || m.router == nil {
		return state
	}
	ctx = context.WithoutCancel(ctx)
	status, err := m.routerRuntime.Status(ctx)
	if err != nil || status.State == runtime.Stopped {
		return state
	}
	state.running = true
	if models, err := m.router.List(ctx); err == nil {
		state.active = activeModelNames(models)
	}
	return state
}

func (m *Manager) restartRouterSources(ctx context.Context, active []string, reason string) (OperationResult, error) {
	result := OperationResult{Action: "restart_router_sources", Target: "router", Status: "succeeded", Message: "router restarted after source change: " + reason}
	if _, err := m.routerRuntime.Stop(ctx); err != nil {
		return result, mapRuntimeError(err, "stop router for source refresh failed")
	}
	if _, err := m.routerRuntime.Start(ctx); err != nil {
		return result, mapRuntimeError(err, "start router after source refresh failed")
	}
	models, err := m.router.List(ctx)
	if err != nil {
		return result, CoreError(ErrorRuntime, "list router sources after restart failed", err)
	}
	if err := m.restoreActiveModels(ctx, active, models); err != nil {
		return result, CoreError(ErrorRuntime, "router restarted; active Preset restore failed", err)
	}
	return result, nil
}

func (m *Manager) restoreActiveModels(ctx context.Context, active []string, models []router.Model) error {
	states := make(map[string]string, len(models))
	for _, model := range models {
		states[model.ID] = model.Status.Value
	}
	var restoreErrors []error
	for _, name := range active {
		state, exists := states[name]
		if !exists {
			restoreErrors = append(restoreErrors, fmt.Errorf("preset %q is no longer available", name))
			continue
		}
		if modelStateActive(state) {
			continue
		}
		if err := m.router.Load(ctx, name); err != nil {
			restoreErrors = append(restoreErrors, fmt.Errorf("reload Preset %q: %w", name, err))
		}
	}
	return errors.Join(restoreErrors...)
}

func activeModelNames(models []router.Model) []string {
	names := make([]string, 0, len(models))
	for _, model := range models {
		if modelStateActive(model.Status.Value) {
			names = append(names, model.ID)
		}
	}
	sort.Strings(names)
	return names
}

func modelStateActive(state string) bool {
	return state == "loaded" || state == "loading" || state == "sleeping" || state == "downloading"
}
