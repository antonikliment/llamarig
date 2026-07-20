package control

import (
	"context"

	"llamarig/core/configstore"
)

func (m *Manager) SetStartupServices(ctx context.Context, services []string) (configstore.WriteResult, error) {
	return withMutationResult(m, ctx, "set_startup_services", configstore.WriteResult{}, func() (configstore.WriteResult, error) {
		if m.config == nil {
			return configstore.WriteResult{}, Errorf(ErrorInvalidInput, "config.yaml store is not configured")
		}
		result, err := m.config.SetStartupServices(ctx, services)
		return result, mapConfigStoreError(err)
	})
}

func (m *Manager) SetPresetAutostart(ctx context.Context, name string, enabled bool) (configstore.WriteResult, error) {
	return withMutationResult(m, ctx, "set_preset_autostart", configstore.WriteResult{}, func() (configstore.WriteResult, error) {
		if m.config == nil {
			return configstore.WriteResult{}, Errorf(ErrorInvalidInput, "config.yaml store is not configured")
		}
		if enabled {
			if _, err := m.GetPreset(ctx, name); err != nil {
				return configstore.WriteResult{}, err
			}
		}
		result, err := m.config.SetRouterAutostartPreset(ctx, name, enabled)
		return result, mapConfigStoreError(err)
	})
}
