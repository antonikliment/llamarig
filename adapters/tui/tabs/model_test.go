package tabs

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
