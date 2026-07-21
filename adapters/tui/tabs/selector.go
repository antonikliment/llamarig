package tabs

import (
	"context"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

type Tab int

const (
	TabServices Tab = iota
	TabModels
	TabSystem
	TabLogs
)

type Model struct {
	active     Tab
	services   ServicesTab
	models     ModelsTab
	system     SystemTab
	logs       LogsTab
	keys       KeyMap
	backend    dashboardBackend
	snapshot   dashboardSnapshot
	refreshing bool
	notice     string
}

func NewModel(ctx context.Context) Model {
	return Model{services: NewServicesTab(), models: NewModelsTab(), system: NewSystemTab(), logs: NewLogsTab(), keys: DefaultKeyMap(), backend: newDashboardBackend(ctx), snapshot: dashboardSnapshot{warnings: map[string]string{}}}
}

func (m *Model) Init() tea.Cmd { return tea.Batch(m.backend.autostart(), m.refresh(), tickDashboard()) }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	case dashboardTickMsg:
		return tea.Batch(m.refresh(), tickDashboard())
	case pollResult:
		m.snapshot, m.refreshing = mergeSnapshot(m.snapshot, msg), false
	case actionRequestMsg:
		cmd := m.backend.run(msg, m.snapshot.config)
		if (msg.target == actionDaemon || msg.target == actionGateway) && msg.index == 1 {
			return tea.Batch(cmd, m.services.spin.Tick)
		}
		return cmd
	case spinner.TickMsg:
		return m.advanceSpinner(msg)
	case actionResultMsg:
		m.services.setResult(msg)
		return m.refresh()
	case presetStartRequestMsg:
		return m.backend.startPreset(msg.name)
	case presetStartResultMsg:
		m.models.setResult(msg)
		return m.refresh()
	default:
		return m.updateModelMessage(msg)
	}
	return nil
}

func (m *Model) updateModelMessage(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case modelDeleteRequestMsg:
		return m.backend.deleteLocalModel(msg.path)
	case modelDeleteResultMsg:
		m.models.setDeleteResult(msg)
		return m.refresh()
	case presetCleanupRequestMsg:
		return m.backend.cleanupPreset(msg.name)
	case presetCleanupResultMsg:
		m.models.setCleanupResult(msg)
		return m.refresh()
	case presetAutostartRequestMsg:
		return m.backend.setPresetAutostart(msg.name, msg.enabled)
	case presetAutostartResultMsg:
		m.services.setAutostartResult(msg)
		return m.refresh()
	case autostartResultMsg:
		return m.updateAutostart(msg)
	}
	return nil
}

func (m *Model) advanceSpinner(msg tea.Msg) tea.Cmd {
	if !m.services.anyStopping() {
		return nil
	}
	var cmd tea.Cmd
	m.services.spin, cmd = m.services.spin.Update(msg)
	return cmd
}

func (m *Model) updateAutostart(msg autostartResultMsg) tea.Cmd {
	m.notice = autostartNotice(msg)
	if len(msg.started) > 0 {
		return m.refresh()
	}
	return nil
}

func autostartNotice(msg autostartResultMsg) string {
	parts := make([]string, 0, len(msg.started)+len(msg.errs))
	for _, name := range msg.started {
		parts = append(parts, "started "+name)
	}
	for name, errText := range msg.errs {
		parts = append(parts, "failed to start "+name+": "+errText)
	}
	return strings.Join(parts, "; ")
}

func (m *Model) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.active != TabLogs || !m.logs.IsSearching() {
		switch {
		case key.Matches(msg, m.keys.ServicesTab):
			m.active = TabServices
		case key.Matches(msg, m.keys.ModelsTab):
			m.active = TabModels
		case key.Matches(msg, m.keys.SystemTab):
			m.active = TabSystem
		case key.Matches(msg, m.keys.LogsTab):
			m.active = TabLogs
		case key.Matches(msg, m.keys.Refresh):
			return m.refresh()
		}
	}
	switch m.active {
	case TabServices:
		return m.services.Update(msg)
	case TabModels:
		return m.models.Update(msg, m.snapshot)
	case TabSystem:
		m.system.Update(msg, m.keys)
	case TabLogs:
		m.logs.Update(msg, m.keys)
	}
	return nil
}

func (m *Model) refresh() tea.Cmd {
	if m.refreshing {
		return nil
	}
	m.refreshing = true
	return m.backend.poll()
}

type dashboardTickMsg struct{}

func tickDashboard() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return dashboardTickMsg{} })
}

func (m *Model) ActiveTab() Tab           { return m.active }
func (m *Model) LastRefreshed() time.Time { return m.snapshot.refreshed }
func (m *Model) Notice() string           { return m.notice }
func (m *Model) FooterWarning() string {
	if len(m.snapshot.warnings) == 0 {
		return ""
	}
	names := make([]string, 0, len(m.snapshot.warnings))
	for name := range m.snapshot.warnings {
		names = append(names, name)
	}
	sort.Strings(names)
	for index, name := range names {
		names[index] = name + ": " + m.snapshot.warnings[name]
	}
	return strings.Join(names, ", ")
}
func (m *Model) View(width, height int) string {
	switch m.active {
	case TabModels:
		return m.models.View(width, height, m.snapshot)
	case TabSystem:
		return m.system.View(width, height, m.snapshot)
	case TabLogs:
		return m.logs.View(width, height, m.snapshot)
	default:
		return m.services.View(width, height, m.snapshot)
	}
}
