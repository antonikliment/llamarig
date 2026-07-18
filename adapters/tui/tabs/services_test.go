package tabs

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/platform/process"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestServicesTabCyclesPanelsAndActions(t *testing.T) {
	tab := NewServicesTab()
	tab.Update(keyMsg("tab"))
	if tab.focus != servicePanelHTTP {
		t.Fatalf("focus = %d", tab.focus)
	}
	tab.Update(keyMsg("right"))
	if tab.selected[servicePanelHTTP] != 1 {
		t.Fatalf("selected = %d", tab.selected[servicePanelHTTP])
	}
	for range servicePanelCount - 1 {
		tab.Update(keyMsg("tab"))
	}
	if tab.focus != servicePanelDaemon {
		t.Fatalf("wrapped focus = %d", tab.focus)
	}
}

func keyMsg(value string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: value, Code: []rune(value)[0]})
}

func TestQuitBindingUsesOnlyControlC(t *testing.T) {
	keys := DefaultKeyMap()
	if !key.Matches(keyMsg("ctrl+c"), keys.Quit) || key.Matches(keyMsg("q"), keys.Quit) || key.Matches(keyMsg("esc"), keys.Quit) {
		t.Fatal("quit binding must use only Ctrl+C")
	}
}

func TestServicesActionCreatesRequestOnlyForActionablePanels(t *testing.T) {
	tab := NewServicesTab()
	msg := tab.Update(keyMsg("enter"))()
	request, ok := msg.(actionRequestMsg)
	if !ok || request.target != actionDaemon {
		t.Fatalf("request = %#v", msg)
	}
	tab.focus = servicePanelModels
	tab.View(160, 40, dashboardSnapshot{runtime: &controlv1.RuntimeStatus{Presets: []*controlv1.RuntimePreset{
		{Name: "chat", State: "running"}, {Name: "embedding", State: "running"},
	}}, warnings: map[string]string{}})
	tab.Update(keyMsg("right"))
	runtimeRequest := tab.Update(keyMsg("enter"))().(actionRequestMsg)
	if runtimeRequest.target != actionRuntime || runtimeRequest.name != "embedding" {
		t.Fatalf("runtime request = %#v", runtimeRequest)
	}
}

func TestDaemonStopLocksOnlyDaemonPanel(t *testing.T) {
	tab := NewServicesTab()
	tab.selected[servicePanelDaemon] = 1
	request := tab.Update(keyMsg("enter"))().(actionRequestMsg)
	if request.target != actionDaemon || !tab.stopping[servicePanelDaemon] {
		t.Fatalf("request=%#v stopping=%v", request, tab.stopping)
	}
	tab.Update(keyMsg("left"))
	if tab.selected[servicePanelDaemon] != 1 {
		t.Fatal("daemon action changed while locked")
	}
	if !tab.animateShutdown() || tab.frame[servicePanelDaemon] != 1 {
		t.Fatalf("frame=%v stopping=%v", tab.frame, tab.stopping)
	}
	tab.Update(keyMsg("tab"))
	if tab.focus != servicePanelHTTP {
		t.Fatal("panel navigation was locked")
	}
	tab.setResult(actionResultMsg{target: actionDaemon, index: 1})
	if tab.stopping[servicePanelDaemon] {
		t.Fatal("daemon panel remained locked")
	}
}

func TestDaemonViewShowsShutdownAnimation(t *testing.T) {
	tab := NewServicesTab()
	tab.stopping[servicePanelDaemon], tab.frame[servicePanelDaemon] = true, 2
	view := tab.View(160, 40, dashboardSnapshot{daemon: process.DetachedStatus{Running: true}, warnings: map[string]string{}})
	for _, want := range []string{"Shutting down..", "Controls locked while stopping"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
}

func TestGatewayStopLocksAndAnimatesGatewayPanel(t *testing.T) {
	tab := NewServicesTab()
	tab.focus, tab.selected[servicePanelHTTP] = servicePanelHTTP, 1
	request := tab.Update(keyMsg("enter"))().(actionRequestMsg)
	if request.target != actionGateway || !tab.stopping[servicePanelHTTP] {
		t.Fatalf("request=%#v stopping=%v", request, tab.stopping)
	}
	tab.animateShutdown()
	view := tab.View(160, 40, dashboardSnapshot{gateway: process.DetachedStatus{Running: true}, warnings: map[string]string{}})
	for _, want := range []string{"Shutting down.", "Controls locked while stopping"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	tab.setResult(actionResultMsg{target: actionGateway, index: 1})
	if tab.stopping[servicePanelHTTP] {
		t.Fatal("gateway panel remained locked")
	}
}

func TestServiceResultsStayInOwningPanel(t *testing.T) {
	tab := NewServicesTab()
	tab.setResult(actionResultMsg{target: actionGateway, err: errTestServiceAction})
	view := tab.View(160, 40, dashboardSnapshot{warnings: map[string]string{}})
	daemon, http, _ := strings.Cut(view, "HTTP Server")
	if strings.Contains(daemon, errTestServiceAction.Error()) || !strings.Contains(http, errTestServiceAction.Error()) {
		t.Fatalf("gateway error rendered in wrong panel:\n%s", view)
	}
}

var errTestServiceAction = &serviceActionError{}

type serviceActionError struct{}

func (*serviceActionError) Error() string { return "gateway action failed" }

func TestServicesViewUsesSupportedSnapshotData(t *testing.T) {
	tab := NewServicesTab()
	view := tab.View(160, 40, dashboardSnapshot{
		daemon:   process.DetachedStatus{Running: true, PID: 12},
		gateway:  process.DetachedStatus{Running: true},
		runtime:  &controlv1.RuntimeStatus{Presets: []*controlv1.RuntimePreset{{Name: "chat", State: "running"}}},
		warnings: map[string]string{},
	})
	for _, want := range []string{"Core Daemon", "HTTP Server", "MCP endpoint", "/mcp", "Streamable HTTP", "Llama Runtimes", "1 running", "Quick Help"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	for _, unsupported := range []string{"Requests:", "Clients:"} {
		if strings.Contains(view, unsupported) {
			t.Fatalf("unsupported %q", unsupported)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 160 {
			t.Fatalf("line too wide: %d", lipgloss.Width(line))
		}
	}
}

func TestServicePanelWidthUsesThreeWideColumns(t *testing.T) {
	if got, want := servicePanelWidth(160, 2), 52; got != want {
		t.Fatalf("panel width = %d, want %d", got, want)
	}
}

func TestHTTPActionsHighlightOnlyWhenFocused(t *testing.T) {
	snapshot := dashboardSnapshot{warnings: map[string]string{}}
	if view := ansi.Strip(renderHTTP(52, 10, snapshot, 0, false, false, 0, "", "")); strings.Contains(view, "[Start]") {
		t.Fatalf("unfocused actions highlighted:\n%s", view)
	}
	if view := ansi.Strip(renderHTTP(52, 10, snapshot, 0, true, false, 0, "", "")); !strings.Contains(view, "[Start]") {
		t.Fatalf("focused action not highlighted:\n%s", view)
	}
}

func TestMergeSnapshotPreservesFailedSections(t *testing.T) {
	oldRuntime := &controlv1.RuntimeStatus{State: "running"}
	old := dashboardSnapshot{runtime: oldRuntime, resources: &controlv1.RuntimeResources{}, warnings: map[string]string{}}
	got := mergeSnapshot(old, pollResult{snapshot: dashboardSnapshot{warnings: map[string]string{"runtime": "down"}}})
	if got.runtime != oldRuntime || got.resources != old.resources {
		t.Fatal("failed refresh discarded stale data")
	}
}
