package tui

import (
	"context"
	tabs2 "llamarig/adapters/tui/tabs"
	"llamarig/adapters/tui/ui"
	"llamarig/config"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type model struct {
	tabs   tabs2.Model
	width  int
	height int
	keys   tabs2.KeyMap
}

func (m *model) Init() tea.Cmd {
	return m.tabs.Init()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
	}

	if cmd := m.tabs.Update(msg); cmd != nil {
		return m, cmd
	}
	return m, nil
}

func (m *model) View() tea.View {
	content := m.render()

	view := tea.NewView(content)

	// Bubble Tea v2: use declarative view fields instead of tea.WithAltScreen().
	view.AltScreen = true
	view.WindowTitle = config.ProjectDisplayName + " TUI"

	return view
}

func newModel(ctx context.Context) *model {
	return &model{tabs: tabs2.NewModel(ctx), width: 120, height: 32, keys: tabs2.DefaultKeyMap()}
}

func (m *model) render() string {
	width := m.width
	width = max(width, 20)

	bodyWidth := width - 4
	bodyWidth = max(bodyWidth, 16)

	return ui.AppStyle.Width(width).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			tabs2.RenderHeader(tabs2.ChromeProps{ActiveTab: m.tabs.ActiveTab(), Width: bodyWidth}),
			m.tabs.View(bodyWidth, max(1, m.height-4)),
			tabs2.RenderFooter(tabs2.ChromeProps{Width: bodyWidth, Warning: m.tabs.FooterWarning(), Notice: m.tabs.Notice(), Refreshed: m.tabs.LastRefreshed()}),
		),
	)
}
