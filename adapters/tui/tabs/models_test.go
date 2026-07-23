package tabs

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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
	if cmd := tab.Update(keyMsg("enter"), snapshot); cmd != nil {
		t.Fatalf("start cmd=%v", cmd)
	}
	if view := ansi.Strip(tab.View(120, 20, snapshot)); !strings.Contains(view, "source missing") {
		t.Fatalf("expected source error in view:\n%s", view)
	}
	if cmd := tab.Update(keyMsg("d"), snapshot); cmd != nil || tab.presetStatus.Pending() != "broken" {
		t.Fatalf("first cleanup cmd=%v pending=%q", cmd, tab.presetStatus.Pending())
	}
	cmd := tab.Update(keyMsg("y"), snapshot)
	request, ok := cmd().(presetCleanupRequestMsg)
	if !ok || request.name != "broken" {
		t.Fatalf("request = %#v", request)
	}
	if state := presetState(&snapshot.presets[0], nil); state != "Unavailable" {
		t.Fatalf("state = %q", state)
	}
}

func TestModelsTabNavigationCancelsPendingCleanup(t *testing.T) {
	tab := NewModelsTab()
	snapshot := dashboardSnapshot{presets: []presetView{{Name: "first"}, {Name: "broken", SourceStatus: "unavailable"}}}
	tab.Update(keyMsg("down"), snapshot) // cursor -> 1 (broken, unavailable)
	tab.Update(keyMsg("d"), snapshot)    // arm cleanup for "broken"
	if tab.presetStatus.Pending() != "broken" {
		t.Fatalf("expected armed cleanup, pending=%q", tab.presetStatus.Pending())
	}
	tab.Update(keyMsg("up"), snapshot)
	if tab.presetTable.Cursor() != 0 || tab.presetStatus.Pending() != "" {
		t.Fatalf("cursor=%d pending=%q", tab.presetTable.Cursor(), tab.presetStatus.Pending())
	}
}

func TestModelsTabClampsSelectionAfterPresetsShrink(t *testing.T) {
	tab := NewModelsTab()
	big := dashboardSnapshot{presets: []presetView{{Name: "a"}, {Name: "b"}, {Name: "only"}}}
	tab.Update(keyMsg("down"), big)
	tab.Update(keyMsg("down"), big) // cursor -> 2
	small := dashboardSnapshot{presets: []presetView{{Name: "only"}}}
	cmd := tab.Update(keyMsg("enter"), small)
	request := cmd().(presetStartRequestMsg)
	if request.name != "only" {
		t.Fatalf("request=%q", request.name)
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
	view := ansi.Strip(tab.View(120, 20, dashboardSnapshot{presets: []presetView{*preset}}))
	for _, want := range []string{"Enter Run", "chat", "chat.gguf", "Stopped"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
}
