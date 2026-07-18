package control

import (
	"context"

	"llamarig/core/configstore"
)

func (m *Manager) GetConfigYAML(ctx context.Context) (configstore.ConfigYAML, error) {
	if m.config == nil {
		return configstore.ConfigYAML{}, Errorf(ErrorInvalidInput, "config.yaml store is not configured")
	}
	cfg, err := m.config.Read(ctx)
	return cfg, mapConfigStoreError(err)
}

func (m *Manager) ValidateConfigYAML(ctx context.Context, content string) error {
	if m.config == nil {
		return Errorf(ErrorInvalidInput, "config.yaml store is not configured")
	}
	return mapConfigStoreError(m.config.Validate(ctx, content))
}

func (m *Manager) ReplaceConfigYAML(ctx context.Context, content string) (configstore.WriteResult, error) {
	return withMutationResult(m, ctx, "replace_config_yaml", configstore.WriteResult{}, func() (configstore.WriteResult, error) {
		if m.config == nil {
			return configstore.WriteResult{}, Errorf(ErrorInvalidInput, "config.yaml store is not configured")
		}
		result, err := m.config.Replace(ctx, content)
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
