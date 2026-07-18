package rpc

import (
	"context"
	"path/filepath"
	"testing"

	"llamarig/core/control"
	"llamarig/core/modelpresets"
	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestPresetRPCRejectsINIInjection(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	svc := NewControlService(RPCDependencies{Manager: control.NewManager(control.Dependencies{Presets: store})})
	for _, entry := range []*controlv1.PresetEntry{
		{Key: "model\n[other]", Value: "/model.gguf"},
		{Key: "model", Value: "/model.gguf\n[other]\nmodel = /other.gguf"},
		{Key: "Model", Value: "/model.gguf"},
	} {
		_, err := svc.PutPreset(context.Background(), &controlv1.PutPresetRequest{
			Preset: &controlv1.ModelPreset{Name: "demo", Entries: []*controlv1.PresetEntry{entry}},
		})
		if ErrorKindFromRPC(err) != control.ErrorInvalidInput {
			t.Fatalf("entry %#v: error = %v", entry, err)
		}
	}
}

func TestPresetRPCDeleteMissingIsSuccessful(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	events := control.NewEventStore(10)
	svc := NewControlService(RPCDependencies{Manager: control.NewManager(control.Dependencies{Presets: store, Audit: events})})

	resp, err := svc.DeletePreset(context.Background(), &controlv1.DeletePresetRequest{Name: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetOk() {
		t.Fatalf("response = %#v", resp)
	}
	if got := events.List(); len(got) != 1 || got[0].Action != "delete_preset" || !got[0].Success {
		t.Fatalf("events = %#v", got)
	}
}

func TestCleanupPresetRejectsNilRequest(t *testing.T) {
	svc := NewControlService(RPCDependencies{Manager: control.NewManager(control.Dependencies{})})
	_, err := svc.CleanupPreset(context.Background(), nil)
	if ErrorKindFromRPC(err) != control.ErrorInvalidInput {
		t.Fatalf("CleanupPreset() error = %v", err)
	}
}

func TestPresetEntriesSortModelSourcesFirst(t *testing.T) {
	entries := presetEntries(modelpresets.Section{Values: map[string]string{
		"ctx-size": "4096", "models-dir": "/models", "model": "/model.gguf",
	}})
	if entries[0].GetKey() != "model" || entries[1].GetKey() != "models-dir" || entries[2].GetKey() != "ctx-size" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestSectionFromProtoHandlesNil(t *testing.T) {
	section := sectionFromProto(nil)
	if section.Name != "" || section.Values != nil {
		t.Fatalf("section = %#v", section)
	}
}
