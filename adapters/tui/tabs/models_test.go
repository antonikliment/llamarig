package tabs

import (
	"strings"
	"testing"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestModelsTabSelectsAndStartsStoppedPreset(t *testing.T) {
	tab := NewModelsTab()
	snapshot := dashboardSnapshot{presets: []presetView{{Name: "a"}, {Name: "b"}}}
	tab.Update(keyMsg("down"), snapshot)
	cmd := tab.Update(keyMsg("enter"), snapshot)
	request, ok := cmd().(presetStartRequestMsg)
	if !ok || request.name != "b" {
		t.Fatalf("request = %#v", request)
	}
}

func TestModelsTabDoesNotStartRunningPreset(t *testing.T) {
	tab := NewModelsTab()
	snapshot := dashboardSnapshot{
		presets: []presetView{{Name: "chat"}},
		runtime: &controlv1.RuntimeStatus{Presets: []*controlv1.RuntimePreset{{Name: "chat", State: "running"}}},
	}
	if cmd := tab.Update(keyMsg("enter"), snapshot); cmd != nil {
		t.Fatalf("running preset command = %v", cmd)
	}
}

func TestModelsTabShowsAndConfirmsUnavailablePresetCleanup(t *testing.T) {
	tab := NewModelsTab()
	snapshot := dashboardSnapshot{presets: []presetView{{Name: "broken", Model: "/missing.gguf", SourceStatus: "unavailable", SourceError: "source missing"}}}
	if cmd := tab.Update(keyMsg("enter"), snapshot); cmd != nil || tab.err != "source missing" {
		t.Fatalf("start cmd=%v err=%q", cmd, tab.err)
	}
	if cmd := tab.Update(keyMsg("d"), snapshot); cmd != nil || tab.pendingCleanup != "broken" {
		t.Fatalf("first cleanup cmd=%v pending=%q", cmd, tab.pendingCleanup)
	}
	cmd := tab.Update(keyMsg("y"), snapshot)
	request, ok := cmd().(presetCleanupRequestMsg)
	if !ok || request.name != "broken" {
		t.Fatalf("request = %#v", request)
	}
	if row := presetRow(&snapshot.presets[0], nil, false, 120); !strings.Contains(row, "Unavailable") {
		t.Fatalf("row = %q", row)
	}
}

func TestModelsTabNavigationCancelsPendingCleanup(t *testing.T) {
	tab := NewModelsTab()
	tab.selected, tab.pendingCleanup, tab.message, tab.err = 1, "broken", "stale success", "stale error"
	snapshot := dashboardSnapshot{presets: []presetView{{Name: "first"}, {Name: "broken", SourceStatus: "unavailable"}}}
	tab.Update(keyMsg("up"), snapshot)
	if tab.selected != 0 || tab.pendingCleanup != "" || tab.message != "" || tab.err != "" {
		t.Fatalf("selected=%d pending=%q message=%q err=%q", tab.selected, tab.pendingCleanup, tab.message, tab.err)
	}
}

func TestModelsTabClampsSelectionAfterPresetsShrink(t *testing.T) {
	tab := NewModelsTab()
	tab.selected = 2
	snapshot := dashboardSnapshot{presets: []presetView{{Name: "only"}}}
	cmd := tab.Update(keyMsg("enter"), snapshot)
	request := cmd().(presetStartRequestMsg)
	if request.name != "only" || tab.selected != 0 {
		t.Fatalf("selected=%d request=%q", tab.selected, request.name)
	}
}

func TestRuntimePresetLine(t *testing.T) {
	line := runtimePresetLine(&controlv1.RuntimePreset{Name: "embed"}, false)
	if !strings.Contains(line, "embed") {
		t.Fatalf("line = %q", line)
	}
}

func TestModelsViewShowsPresetFieldsAndHelp(t *testing.T) {
	tab := NewModelsTab()
	preset := &presetView{Name: "chat", Model: "/models/chat.gguf"}
	view := tab.View(120, 20, dashboardSnapshot{presets: []presetView{*preset}})
	for _, want := range []string{"Quick Help", "Enter Run"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	row := presetRow(preset, nil, false, 120)
	for _, want := range []string{"chat", "chat.gguf", "Stopped"} {
		if !strings.Contains(row, want) {
			t.Fatalf("row missing %q: %s", want, row)
		}
	}
}
