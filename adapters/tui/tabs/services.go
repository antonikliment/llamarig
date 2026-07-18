package tabs

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"llamarig/adapters/tui/ui"
	"llamarig/config"
	controlv1 "llamarig/core/rpc/gen/v1"

	bindkey "charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type servicePanel int

const (
	servicePanelDaemon servicePanel = iota
	servicePanelHTTP
	servicePanelModels
	servicePanelCount
)

type ServicesTab struct {
	focus           servicePanel
	selected        [servicePanelCount]int
	message         [servicePanelCount]string
	err             [servicePanelCount]string
	keys            KeyMap
	scroll          int
	stopping        [servicePanelCount]bool
	frame           [servicePanelCount]int
	runtimes        []string
	presetAutostart map[string]bool
}

func NewServicesTab() ServicesTab { return ServicesTab{keys: DefaultKeyMap()} }

func (t *ServicesTab) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch {
	case bindkey.Matches(key, t.keys.NextPanel):
		t.focus = (t.focus + 1) % servicePanelCount
	case bindkey.Matches(key, t.keys.PreviousPanel):
		t.focus = (t.focus + servicePanelCount - 1) % servicePanelCount
	case bindkey.Matches(key, t.keys.PreviousAction) && !t.controlLocked():
		t.moveAction(-1)
	case bindkey.Matches(key, t.keys.NextAction) && !t.controlLocked():
		t.moveAction(1)
	case bindkey.Matches(key, t.keys.RunAction) && !t.controlLocked():
		return t.action()
	case bindkey.Matches(key, t.keys.ToggleAutostart):
		return t.toggleAutostart()
	default:
		t.updateScroll(key)
	}
	return nil
}

func (t *ServicesTab) updateScroll(key tea.KeyPressMsg) {
	switch {
	case bindkey.Matches(key, t.keys.Up):
		t.scroll = max(0, t.scroll-1)
	case bindkey.Matches(key, t.keys.Down):
		t.scroll++
	}
}

func (t *ServicesTab) moveAction(delta int) {
	count := t.actionCount()
	if count > 0 {
		t.selected[t.focus] = (t.selected[t.focus] + count + delta) % count
	}
}

func (t *ServicesTab) actionCount() int {
	if t.focus == servicePanelModels {
		return len(t.runtimes)
	}
	return 3
}

func (t *ServicesTab) action() tea.Cmd {
	if t.actionCount() == 0 {
		return nil
	}
	request := actionRequestMsg{target: actionTarget(t.focus), index: t.selected[t.focus]}
	if request.target == actionRuntime {
		request.name = t.runtimes[request.index]
	}
	if request.target != actionRuntime && request.index == 1 {
		t.stopping[t.focus], t.frame[t.focus] = true, 0
	}
	t.message[t.focus], t.err[t.focus] = "", ""
	return func() tea.Msg { return request }
}

func (t *ServicesTab) toggleAutostart() tea.Cmd {
	if t.focus != servicePanelModels || len(t.runtimes) == 0 {
		return nil
	}
	name := t.runtimes[t.selected[servicePanelModels]]
	enabled := !t.presetAutostart[name]
	t.message[servicePanelModels], t.err[servicePanelModels] = "", ""
	return func() tea.Msg { return presetAutostartRequestMsg{name: name, enabled: enabled} }
}

func (t *ServicesTab) setAutostartResult(result presetAutostartResultMsg) {
	if result.err != nil {
		t.message[servicePanelModels], t.err[servicePanelModels] = "", result.err.Error()
		return
	}
	msg := "autostart disabled"
	if result.enabled {
		msg = "autostart enabled"
	}
	t.message[servicePanelModels], t.err[servicePanelModels] = msg, ""
}

func (t *ServicesTab) setResult(result actionResultMsg) {
	panel := servicePanel(result.target)
	if result.target != actionRuntime && result.index == 1 {
		t.stopping[panel] = false
	}
	t.message[panel], t.err[panel] = "completed", ""
	if result.err != nil {
		t.message[panel], t.err[panel] = "", result.err.Error()
	}
}

func (t *ServicesTab) View(width, height int, snapshot dashboardSnapshot) string {
	const gap = 2
	t.syncRuntimes(snapshot.runtime, snapshot.presets)
	target := ""
	if len(t.runtimes) > 0 {
		target = t.runtimes[t.selected[servicePanelModels]]
	}
	panelWidth := servicePanelWidth(width, gap)
	top := []string{
		renderDaemon(panelWidth, 10, snapshot, t.selected[servicePanelDaemon], t.focus == servicePanelDaemon, t.stopping[servicePanelDaemon], t.frame[servicePanelDaemon], t.message[servicePanelDaemon], t.err[servicePanelDaemon]),
		renderHTTP(panelWidth, 10, snapshot, t.selected[servicePanelHTTP], t.focus == servicePanelHTTP, t.stopping[servicePanelHTTP], t.frame[servicePanelHTTP], t.message[servicePanelHTTP], t.err[servicePanelHTTP]),
		renderRuntime(panelWidth, 10, snapshot.runtime, snapshot.warnings["runtime"], t.focus == servicePanelModels, target, len(t.runtimes), t.presetAutostart, t.message[servicePanelModels], t.err[servicePanelModels]),
	}
	content := lipgloss.JoinVertical(lipgloss.Left, ui.Flow(width, gap, top), servicesOverview(width, gap, panelWidth, t.keys, t.focus))
	t.scroll = min(t.scroll, max(0, strings.Count(content, "\n")+1-height))
	return ui.VerticalSlice(content, t.scroll, height)
}

// stoppingState resolves the display state/color for a service panel that
// can be running, stopped, or mid-shutdown (animated by frame).
func stoppingState(stopping, running bool, frame int) (string, color.Color) {
	switch {
	case stopping:
		return "Shutting down" + strings.Repeat(".", frame%4), ui.Yellow
	case running:
		return "Running", ui.Green
	default:
		return "Stopped", ui.Muted
	}
}

func renderDaemon(width, height int, snapshot dashboardSnapshot, selected int, focused, stopping bool, frame int, message, actionErr string) string {
	status := snapshot.daemon
	state, stateColor := stoppingState(stopping, status.Running, frame)
	pid, uptime := "-", "-"
	if status.Running {
		pid, uptime = fmt.Sprint(status.PID), status.Uptime.String()
	}
	return renderServicePanel(width, height, "Core Daemon", ui.Green, state, stateColor,
		[]string{fmt.Sprintf("%-8s %s", "PID:", pid), fmt.Sprintf("%-8s %s", "Uptime:", uptime),
			fmt.Sprintf("%-8s %s", "Config:", truncMiddle(snapshot.configPath, width-14)),
			fmt.Sprintf("%-8s %s", "Log:", truncMiddle(snapshot.logPath, width-14))},
		[]string{"Start", "Stop", "Status"}, selected, focused, stopping, message, actionErr)
}

func (t *ServicesTab) controlLocked() bool { return t.stopping[t.focus] }
func (t *ServicesTab) animateShutdown() bool {
	active := false
	for panel := range t.stopping {
		if t.stopping[panel] {
			t.frame[panel]++
			active = true
		}
	}
	return active
}

func renderHTTP(width, height int, snapshot dashboardSnapshot, selected int, focused, stopping bool, frame int, message, actionErr string) string {
	state, stateColor := stoppingState(stopping, snapshot.gateway.Running, frame)
	address := snapshot.config.ListenAddr
	if address == "" {
		address = "-"
	}
	return renderServicePanel(width, height, "HTTP Server", ui.Blue, state, stateColor,
		[]string{ui.Field("Address", address), ui.Field("Base URL", publicBaseURL(address)),
			ui.Field("MCP endpoint", "/mcp"), ui.Field("MCP transport", "Streamable HTTP")},
		[]string{"Start", "Stop", "Open"}, selected, focused, stopping, message, actionErr)
}

func renderActionPanel(content []string, width, height int, focused bool, message, actionErr string, foreground color.Color) string {
	content = appendStatusRows(content, actionErr, message)
	return ui.PanelStyle(foreground, focused).Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}

// renderServicePanel builds the shared skeleton for stoppable service panels:
// StatusTitle + caller-supplied fields + Rule + ActionRow, with the
// "Controls locked while stopping" override when stopping is true.
func renderServicePanel(width, height int, title string, accent color.Color, state string, stateColor color.Color, fields, actionLabels []string, selected int, focused, stopping bool, message, actionErr string) string {
	content := append([]string{ui.StatusTitle(title, state, accent, stateColor, width)}, fields...)
	content = append(content, ui.Rule(width), ui.ActionRow(accent, selected, actionLabels, focused))
	if stopping {
		content[len(content)-1] = ui.MutedStyle.Render("Controls locked while stopping")
	}
	return renderActionPanel(content, width, height, focused, message, actionErr, accent)
}

func renderRuntime(width, height int, status *controlv1.RuntimeStatus, warning string, focused bool, target string, count int, autostartMap map[string]bool, message, actionErr string) string {
	stateColor := ui.Muted
	if count > 0 {
		stateColor = ui.Green
	}
	content := []string{ui.StatusTitle("Llama Runtimes", fmt.Sprintf("%d running", count), ui.Purple, stateColor, width)}
	if warning != "" {
		content = append(content, warningStyle.Render("Unavailable: control socket"))
	} else if count == 0 {
		content = append(content, ui.MutedStyle.Render("No runtime models"))
	} else {
		for _, preset := range status.GetPresets() {
			if preset.GetState() == "running" {
				content = append(content, runtimePresetLine(preset, autostartMap[preset.GetName()]))
			}
		}
	}
	content = append(content, ui.Rule(width), ui.ActionRow(ui.Purple, 0, []string{"Stop " + target}, focused && target != ""))
	return renderActionPanel(content, width, height, focused, message, actionErr, ui.Purple)
}

func (t *ServicesTab) syncRuntimes(status *controlv1.RuntimeStatus, presets []presetView) {
	t.runtimes = t.runtimes[:0]
	for _, preset := range status.GetPresets() {
		if preset.GetState() == "running" {
			t.runtimes = append(t.runtimes, preset.GetName())
		}
	}
	t.selected[servicePanelModels] %= max(1, len(t.runtimes))
	t.presetAutostart = make(map[string]bool, len(presets))
	for _, p := range presets {
		t.presetAutostart[p.Name] = p.Autostart
	}
}

func runtimePresetLine(preset *controlv1.RuntimePreset, autostart bool) string {
	indicator := "   "
	if autostart {
		indicator = "[A]"
	}
	return strings.TrimSpace("● " + preset.GetName() + "  " + indicator)
}

func servicesOverview(width, gap, panelWidth int, keys KeyMap, focus servicePanel) string {
	showAutostart := focus == servicePanelModels
	if width < 96 {
		return ui.Flow(width, gap, []string{ui.PanelStyle(ui.Muted, false).Width(panelWidth).Height(7).Render(llamaRigContent()), ui.PanelStyle(ui.Muted, false).Width(panelWidth).Height(7).Render(quickHelpContent(keys, showAutostart))})
	}
	inner, column := width-4, (width-5)/2
	columns := []string{
		lipgloss.NewStyle().Width(column).Height(6).Render(llamaRigContent()),
		lipgloss.NewStyle().Width(inner - column - 1).Height(6).PaddingLeft(1).Render(quickHelpContent(keys, showAutostart)),
	}
	separator := ui.MutedStyle.Render(strings.Repeat("│\n", 5) + "│")
	return ui.PanelStyle(ui.Muted, false).Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Top, columns[0], separator, columns[1]))
}

func llamaRigContent() string {
	return lipgloss.JoinVertical(lipgloss.Left, ui.BrandStyle.Render(config.ProjectDisplayName), config.ProjectDisplayName+" is a local AI config server.", "It exposes running and configuring llama.cpp instances,", "with unified HTTP and MCP interfaces.")
}
func quickHelpContent(keys KeyMap, showAutostart bool) string {
	lines := quickHelpLines(keys)
	autostartLine := lines[6]
	if !showAutostart {
		autostartLine = ui.MutedStyle.Render(autostartLine)
	}
	return lipgloss.JoinVertical(lipgloss.Left, ui.MutedStyle.Render("Quick Help"), fmt.Sprintf("%-22s %s", lines[0], lines[1]), fmt.Sprintf("%-22s %s", lines[2], lines[3]), fmt.Sprintf("%-22s %s", lines[4], lines[5]), autostartLine)
}

func servicePanelWidth(total, gap int) int {
	if total <= 28 {
		return max(0, total)
	}
	if total >= 118 {
		return (total - (int(servicePanelCount)-1)*gap) / int(servicePanelCount)
	}
	return adaptivePanelWidth(total, gap, 28, 48)
}
func adaptivePanelWidth(total, gap, minimum, maximum int) int {
	columns := max(1, (total+gap)/(minimum+gap))
	return min(max((total-(columns-1)*gap)/columns, minimum), maximum)
}

// truncMiddle keeps a path on a single panel line by eliding its middle with
// an ellipsis once it exceeds the available width, so labels never orphan onto
// a wrapped line beneath them.
func truncMiddle(path string, width int) string {
	runes := []rune(path)
	if width <= 1 || len(runes) <= width {
		return path
	}
	head := (width - 1) / 2
	return string(runes[:head]) + "…" + string(runes[len(runes)-(width-1-head):])
}

func shortPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
