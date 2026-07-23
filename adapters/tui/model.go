package tui

import (
	"context"

	tabs2 "llamarig/adapters/tui/tabs"
	"llamarig/config"

	tea "charm.land/bubbletea/v2"
	"github.com/antonikliment/tuikit"
)

// model bootstraps the dashboard's tuikit.Frame, which owns Update and View
// from here on. This wrapper exists only to run tabs2.Model's startup command
// once: tuikit.Frame.Init doesn't propagate to pages, but the dashboard needs
// to kick off its backend autostart, first poll, and refresh ticker.
type model struct {
	frame *tuikit.Frame
	tabs  *tabs2.Model
}

func (m *model) Init() tea.Cmd { return m.tabs.Init() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m.frame.Update(msg) }

func (m *model) View() tea.View { return m.frame.View() }

func newModel(ctx context.Context) *model {
	m := tabs2.NewModel(ctx)
	frame := tuikit.New(
		tuikit.WithBrand(config.ProjectDisplayName, "Local AI control service"),
		tuikit.WithPages(tabs2.Pages(&m)...),
		tuikit.WithStatus(tabs2.Status(&m)),
	)
	return &model{frame: frame, tabs: &m}
}
