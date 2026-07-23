package tabs

import (
	tea "charm.land/bubbletea/v2"
	"github.com/antonikliment/tuikit"
)

// tabPage adapts one dashboard sub-tab to tuikit.Page (and InputCapturer for
// the Logs tab's search field), so a tuikit.Frame can own the numbered
// header, footer, and page-switching chrome while every sub-tab keeps
// reading and writing the single shared Model exactly as it does today.
type tabPage struct {
	model *Model
	tab   Tab
	title string
}

func (p *tabPage) Title() string { return p.title }

func (p *tabPage) Update(msg tea.Msg) tea.Cmd {
	p.model.active = p.tab
	return p.model.Update(msg)
}

func (p *tabPage) View(width, height int) string {
	p.model.active = p.tab
	return p.model.View(width, height)
}

// CapturingInput reports true only while the Logs pane's search field has
// focus, so Frame stops treating "1".."4" as page-switch digits and forwards
// them to the search box instead — matching the guard Model.updateKey already
// applies internally when called directly.
func (p *tabPage) CapturingInput() bool {
	return p.tab == TabLogs && p.model.logs.IsSearching()
}

// Pages returns the Frame pages for the dashboard, all sharing m.
func Pages(m *Model) []tuikit.Page {
	return []tuikit.Page{
		&tabPage{model: m, tab: TabServices, title: "Services"},
		&tabPage{model: m, tab: TabModels, title: "Models"},
		&tabPage{model: m, tab: TabSystem, title: "System"},
		&tabPage{model: m, tab: TabLogs, title: "Logs"},
	}
}

// Status returns a Frame status function reflecting m's footer warning or
// notice (in that priority) and its last-refreshed time, combined into
// Frame's single status line.
func Status(m *Model) tuikit.StatusFunc {
	return func() (string, tuikit.Level) {
		text, level := "Ready", tuikit.LevelInfo
		switch {
		case m.FooterWarning() != "":
			text, level = m.FooterWarning(), tuikit.LevelWarning
		case m.Notice() != "":
			text, level = m.Notice(), tuikit.LevelSuccess
		}
		refreshed := "--:--:--"
		if r := m.LastRefreshed(); !r.IsZero() {
			refreshed = r.Format("15:04:05")
		}
		return text + "   Last refreshed: " + refreshed, level
	}
}
