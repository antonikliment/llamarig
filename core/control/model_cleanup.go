package control

import (
	"context"
	"fmt"

	"llamarig/core/modelpresets"
	"llamarig/core/runtime"
)

func (m *Manager) DeleteLocalModel(ctx context.Context, path string, cascade bool) (OperationResult, error) {
	return withMutationResult(m, ctx, "delete_local_model", OperationResult{}, func() (OperationResult, error) {
		if m.localModels == nil {
			return OperationResult{}, Errorf(ErrorInvalidInput, "local model inventory is not configured")
		}
		if err := m.requirePresets(); err != nil {
			return OperationResult{}, err
		}
		presets, err := m.presets.List(ctx)
		if err != nil {
			return OperationResult{}, mapServerConfigError(err)
		}
		refs := modelpresets.FindReferences(presets, path)
		if err := m.validateModelCascade(ctx, refs.ModelPaths, refs.ModelDirs, cascade); err != nil {
			return OperationResult{}, err
		}
		// Delete file first: failed OS deletion preserves both model and Presets;
		// failed cleanup leaves a supported Unavailable Preset for explicit retry.
		if err := m.localModels.DeleteLocal(ctx, path); err != nil {
			return OperationResult{}, err
		}
		if err := m.removeReferencedPresets(ctx, refs.ModelPaths); err != nil {
			return OperationResult{}, err
		}
		result := OperationResult{Action: "delete", Target: path, Status: "succeeded", Message: "local model deleted"}
		if len(refs.ModelPaths) > 0 {
			result.Message = fmt.Sprintf("local model and %d referencing Preset(s) deleted", len(refs.ModelPaths))
		}
		return result, nil
	})
}

func (m *Manager) validateModelCascade(ctx context.Context, modelPaths, modelDirs []string, cascade bool) error {
	if len(modelPaths) > 0 && !cascade {
		return Errorf(ErrorConflict, "model is referenced by Presets %v; confirm cascading cleanup", modelPaths)
	}
	if len(modelPaths)+len(modelDirs) == 0 || m.routerRuntime == nil || m.router == nil {
		return nil
	}
	status, err := m.routerRuntime.Status(ctx)
	if err != nil {
		return mapRuntimeError(err, "router runtime status failed")
	}
	if status.State == runtime.Stopped {
		return nil
	}
	models, err := m.router.List(ctx)
	if err != nil {
		return mapRuntimeError(err, "router status failed")
	}
	active := make(map[string]bool, len(models))
	for _, model := range models {
		active[model.ID] = modelStateActive(model.Status.Value)
	}
	if err := rejectActivePreset(active, modelPaths); err != nil {
		return err
	}
	return rejectActivePreset(active, modelDirs)
}

func rejectActivePreset(active map[string]bool, names []string) error {
	for _, name := range names {
		if active[name] {
			return Errorf(ErrorConflict, "model cannot be deleted while Preset %q is loaded", name)
		}
	}
	return nil
}

func (m *Manager) removeReferencedPresets(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}
	if err := m.removePresetReferences(ctx, names...); err != nil {
		return err
	}
	for _, name := range names {
		if err := m.presets.Delete(ctx, name); err != nil {
			return mapServerConfigError(err)
		}
	}
	_, err := m.refreshRouterSources(ctx, "Preset references removed before model deletion")
	return err
}
