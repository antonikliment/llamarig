package tabs

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Page switching is owned by the tuikit.Frame wrapping this Model (see
// tabs/page.go), not by Model itself. Frame drives it by rendering the active
// Page every frame, so Pages()[i].View syncs Model.active — this is what these
// tests exercise instead of feeding Model raw digit keys directly.
func TestPagesHaveExpectedTitlesAndOrder(t *testing.T) {
	m := NewModel(context.Background())
	pages := Pages(&m)
	want := []string{"Services", "Models", "System", "Logs"}
	for i, title := range want {
		if got := pages[i].Title(); got != title {
			t.Fatalf("pages[%d].Title() = %q, want %q", i, got, title)
		}
	}
}

func TestPagesSyncActiveTabOnView(t *testing.T) {
	m := NewModel(context.Background())
	pages := Pages(&m)
	want := []Tab{TabServices, TabModels, TabSystem, TabLogs}
	for i, tab := range want {
		pages[i].View(80, 20)
		if m.ActiveTab() != tab {
			t.Fatalf("after Pages()[%d].View, ActiveTab() = %v, want %v", i, m.ActiveTab(), tab)
		}
	}
}

func TestLogsPageCapturesInputWhileSearching(t *testing.T) {
	m := NewModel(context.Background())
	logsPage, ok := Pages(&m)[3].(interface{ CapturingInput() bool })
	if !ok {
		t.Fatal("logs page does not implement CapturingInput")
	}
	if logsPage.CapturingInput() {
		t.Fatal("logs page should not capture input before searching")
	}
	m.logs.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}), m.keys)
	if !logsPage.CapturingInput() {
		t.Fatal("logs page should capture input once search is focused")
	}
}

func TestModelDelegatesResourceMessages(t *testing.T) {
	m := NewModel(context.Background())
	m.Update(pollResult{snapshot: dashboardSnapshot{warnings: map[string]string{"resources": "down"}}})

	if !strings.Contains(m.FooterWarning(), "down") {
		t.Fatalf("footer warning = %q", m.FooterWarning())
	}
}
