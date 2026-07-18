package e2e

import (
	"context"
	"slices"
	"testing"

	"llamarig/config"
	"llamarig/core/control"
	"llamarig/core/rpc"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

func TestPresetAutostartLifecycle(t *testing.T) {
	client := startService(t)
	ctx := context.Background()
	name := "e2e-autostart"
	entries := stubPresetEntries(t)

	created, err := client.PutPreset(ctx, &controlv1.PutPresetRequest{Preset: &controlv1.ModelPreset{Name: name, Entries: []*controlv1.PresetEntry{{Key: "models-dir", Value: entries["models-dir"]}}}, CreateOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "PutPreset", created.GetOk())

	setAndAssertAutostart(t, client, name, true)
	setAndAssertAutostart(t, client, name, false)
	setAndAssertAutostart(t, client, name, true)

	if _, err := client.DeletePreset(ctx, &controlv1.DeletePresetRequest{Name: name}); rpc.ErrorKindFromRPC(err) != control.ErrorConflict {
		t.Fatalf("DeletePreset while autostart enabled error = %v", err)
	}
	if _, err := client.SetPresetAutostart(ctx, &controlv1.PresetAutostartRequest{Name: name}); err != nil {
		t.Fatal(err)
	}
	deleted, err := client.DeletePreset(ctx, &controlv1.DeletePresetRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "DeletePreset", deleted.GetOk())
	doc, err := client.GetConfig(ctx, &controlv1.GetConfigRequest{})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse([]byte(doc.GetContent()))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(cfg.Router.AutostartPresets, name) {
		t.Fatalf("deleted preset remains in autostart config: %v", cfg.Router.AutostartPresets)
	}
}

func setAndAssertAutostart(t *testing.T, client controlv1connect.ControlServiceClient, name string, enabled bool) {
	t.Helper()
	ctx := context.Background()
	updated, err := client.SetPresetAutostart(ctx, &controlv1.PresetAutostartRequest{Name: name, Enabled: enabled})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "SetPresetAutostart", updated.GetOk())
	listed, err := client.ListPresets(ctx, &controlv1.ListPresetsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	preset := findListedPreset(listed.GetPresets(), name)
	if preset == nil || preset.GetAutostart() != enabled {
		t.Fatalf("autostart=%v preset=%#v", enabled, preset)
	}
	doc, err := client.GetConfig(ctx, &controlv1.GetConfigRequest{})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse([]byte(doc.GetContent()))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(cfg.Router.AutostartPresets, name) != enabled {
		t.Fatalf("autostart=%v config=%v", enabled, cfg.Router.AutostartPresets)
	}
}

func findListedPreset(presets []*controlv1.ModelPreset, name string) *controlv1.ModelPreset {
	for _, preset := range presets {
		if preset.GetName() == name {
			return preset
		}
	}
	return nil
}
