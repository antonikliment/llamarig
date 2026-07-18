package tabs

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestModelSwitchesTabs(t *testing.T) {
	m := NewModel(context.Background())

	m.Update(tea.KeyPressMsg(tea.Key{Text: "3", Code: '3'}))
	if m.ActiveTab() != TabSystem {
		t.Fatalf("active tab = %v, want %v", m.ActiveTab(), TabSystem)
	}
	m.Update(tea.KeyPressMsg(tea.Key{Text: "2", Code: '2'}))
	if m.ActiveTab() != TabModels {
		t.Fatalf("active tab = %v, want %v", m.ActiveTab(), TabModels)
	}
}

func TestModelDelegatesResourceMessages(t *testing.T) {
	m := NewModel(context.Background())
	m.Update(pollResult{snapshot: dashboardSnapshot{warnings: map[string]string{"resources": "down"}}})

	if !strings.Contains(m.FooterWarning(), "down") {
		t.Fatalf("footer warning = %q", m.FooterWarning())
	}
}

func TestFooterRendersActualWarning(t *testing.T) {
	view := RenderFooter(ChromeProps{Width: 80, Warning: "presets: unavailable"})
	if !strings.Contains(view, "Status: presets: unavailable") {
		t.Fatalf("footer = %q", view)
	}
}

func TestHeaderShowsControlCQuit(t *testing.T) {
	view := strings.Join(strings.Fields(ansi.Strip(RenderHeader(ChromeProps{ActiveTab: TabServices, Width: 80}))), " ")
	if !strings.Contains(view, "Ctrl+C Quit") {
		t.Fatalf("header missing Ctrl+C quit help: %q", view)
	}
	if strings.Contains(view, "q: quit") {
		t.Fatalf("header shows stale q quit help: %q", view)
	}
}
