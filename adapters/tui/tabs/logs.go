package tabs

import (
	"encoding/json"
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"
	"time"

	"llamarig/adapters/tui/ui"
	"llamarig/config"
	"llamarig/platform/audit"

	bindkey "charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const maxLogLines = 2000

// zapEntry is a parsed line from the daemon's zap JSON log output.
type zapEntry struct {
	Level      string
	Time       float64
	Msg        string
	Caller     string
	Stacktrace string
	Fields     map[string]any
}

// readDaemonLog tails the daemon log file and splits it into parsed zap
// entries (daemon output) and raw lines (interleaved llama-server child
// stdout/stderr), since both are written to the same file.
func readDaemonLog() ([]zapEntry, []string, error) {
	text, err := audit.TailLogLines(config.ProjectName, maxLogLines)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxLogLines {
		lines = lines[len(lines)-maxLogLines:]
	}

	daemonLog := make([]zapEntry, 0, len(lines))
	llamaLog := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if entry, ok := parseZapLine(line); ok {
			daemonLog = append(daemonLog, entry)
		} else {
			llamaLog = append(llamaLog, line)
		}
	}
	return daemonLog, llamaLog, nil
}

func parseZapLine(line string) (zapEntry, bool) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return zapEntry{}, false
	}
	level, ok := raw["level"].(string)
	if !ok || level == "" {
		return zapEntry{}, false
	}
	entry := zapEntry{Level: level, Fields: map[string]any{}}
	if ts, ok := raw["ts"].(float64); ok {
		entry.Time = ts
	}
	if msg, ok := raw["msg"].(string); ok {
		entry.Msg = msg
	}
	if caller, ok := raw["caller"].(string); ok {
		entry.Caller = caller
	}
	if trace, ok := raw["stacktrace"].(string); ok {
		entry.Stacktrace = trace
	}
	for key, value := range raw {
		switch key {
		case "level", "ts", "msg", "caller", "stacktrace":
		default:
			entry.Fields[key] = value
		}
	}
	return entry, true
}

type logPane int

const (
	paneDaemon logPane = iota
	paneLlama
	paneCount
)

type LogsTab struct {
	focus  logPane
	vp     [paneCount]viewport.Model
	follow [paneCount]bool
	input  [paneCount]textinput.Model
}

func NewLogsTab() LogsTab {
	t := LogsTab{follow: [paneCount]bool{true, true}}
	for i := range t.input {
		t.input[i] = textinput.New()
		t.vp[i] = viewport.New()
	}
	return t
}

func (t *LogsTab) IsSearching() bool { return t.input[t.focus].Focused() }

func (t *LogsTab) Update(msg tea.Msg, keys KeyMap) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	if t.input[t.focus].Focused() {
		if s := key.String(); s == "enter" || s == "esc" {
			t.input[t.focus].Blur()
		} else {
			t.input[t.focus], _ = t.input[t.focus].Update(key)
		}
		return
	}
	switch {
	case bindkey.Matches(key, keys.NextPanel):
		t.focus = (t.focus + 1) % paneCount
	case bindkey.Matches(key, keys.PreviousPanel):
		t.focus = (t.focus + paneCount - 1) % paneCount
	case key.String() == "/":
		t.input[t.focus].Focus()
	case key.String() == "esc":
		t.input[t.focus].SetValue("")
		t.follow[t.focus] = true
	case bindkey.Matches(key, keys.Up):
		t.vp[t.focus].ScrollUp(1)
		t.follow[t.focus] = t.vp[t.focus].AtBottom()
	case bindkey.Matches(key, keys.Down):
		t.vp[t.focus].ScrollDown(1)
		t.follow[t.focus] = t.vp[t.focus].AtBottom()
	}
}

// logPaneMeta describes each log as a switchable sub-tab.
var logPaneMeta = [paneCount]struct {
	title  string
	accent color.Color
}{
	paneDaemon: {"Daemon — zap", ui.Green},
	paneLlama:  {"Llama server", ui.Cyan},
}

func (t *LogsTab) View(width, height int, snapshot dashboardSnapshot) string {
	const helpHeight = 3
	tabbedH := max(6, height-helpHeight)

	lines := [paneCount][]string{
		paneDaemon: renderDaemonLog(filterDaemonLog(snapshot.daemonLog, t.input[paneDaemon].Value())),
		paneLlama:  renderLlamaLog(filterLlamaLog(snapshot.llamaLog, t.input[paneLlama].Value())),
	}

	titles := make([]string, paneCount)
	accents := make([]color.Color, paneCount)
	for pane := logPane(0); pane < paneCount; pane++ {
		titles[pane] = fmt.Sprintf("%s (%d)", logPaneMeta[pane].title, len(lines[pane]))
		accents[pane] = logPaneMeta[pane].accent
	}

	vp := &t.vp[t.focus]
	vp.SetWidth(width - 4)
	vp.SetHeight(max(1, tabbedH-4)) // tab row (2) + notch line (1) + box bottom border (1)
	vp.SetContent(strings.Join(lines[t.focus], "\n"))
	if t.follow[t.focus] {
		vp.GotoBottom()
	}

	tabbed := ui.TabbedPanel(titles, accents, int(t.focus), width, tabbedH, vp.View())
	body := lipgloss.JoinVertical(lipgloss.Left, tabbed, logsHelp(width, t))
	return lipgloss.NewStyle().MaxHeight(height).Render(body)
}

var (
	logSwitchKey = bindkey.NewBinding(bindkey.WithKeys("tab"), bindkey.WithHelp("Tab", "Switch log"))
	logScrollKey = bindkey.NewBinding(bindkey.WithKeys("up", "down"), bindkey.WithHelp("↑/↓", "Scroll"))
	logSearchKey = bindkey.NewBinding(bindkey.WithKeys("/"), bindkey.WithHelp("/", "Search"))
	logClearKey  = bindkey.NewBinding(bindkey.WithKeys("esc"), bindkey.WithHelp("Esc", "Clear"))
	logTabKey    = bindkey.NewBinding(bindkey.WithKeys("1", "2", "3", "4"), bindkey.WithHelp("1/2/3/4", "Switch tab"))
)

func logsHelp(width int, t *LogsTab) string {
	status := helpLine(logSwitchKey, logScrollKey, logSearchKey, logClearKey, logTabKey)
	if t.input[t.focus].Focused() {
		status = ui.MutedStyle.Render("Search: " + t.input[t.focus].View() + "  (Enter/Esc to finish)")
	} else if query := t.input[t.focus].Value(); query != "" {
		status = ui.MutedStyle.Render("Search: " + query + "  (Esc to clear)")
	}
	return ui.PanelStyle(ui.Muted, false).Width(width).Render(status)
}

func filterLlamaLog(lines []string, query string) []string {
	if query == "" {
		return lines
	}
	out := make([]string, 0, len(lines))
	needle := strings.ToLower(query)
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), needle) {
			out = append(out, line)
		}
	}
	return out
}

func filterDaemonLog(entries []zapEntry, query string) []zapEntry {
	if query == "" {
		return entries
	}
	needle := strings.ToLower(query)
	out := make([]zapEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Msg+" "+entry.Caller+" "+entry.Stacktrace+" "+renderFields(entry.Fields)), needle) {
			out = append(out, entry)
		}
	}
	return out
}

func renderDaemonLog(entries []zapEntry) []string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, renderZapLine(entry)...)
	}
	return lines
}

func renderZapLine(e zapEntry) []string {
	ts := ui.MutedStyle.Render(time.Unix(int64(e.Time), 0).Format("15:04:05"))
	level, levelStyle := strings.ToUpper(e.Level), levelStyleFor(e.Level)
	line := ts + "  " + levelStyle.Render(fmt.Sprintf("%-5s", level)) + "  " + e.Msg
	if fields := renderFields(e.Fields); fields != "" {
		line += "  " + ui.MutedStyle.Render(fields)
	}
	if e.Stacktrace == "" {
		return []string{line}
	}
	frame := strings.SplitN(e.Stacktrace, "\n", 2)[0]
	return []string{line, ui.MutedStyle.Render("  ↳ " + frame)}
}

func levelStyleFor(level string) lipgloss.Style {
	switch level {
	case "warn":
		return warningStyle
	case "error", "fatal", "dpanic", "panic":
		return lipgloss.NewStyle().Foreground(ui.Red)
	case "debug":
		return ui.MutedStyle
	default:
		return ui.GreenStyle
	}
}

func renderFields(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+formatFieldValue(fields[key]))
	}
	return strings.Join(parts, " ")
}

func formatFieldValue(value any) string {
	text := fmt.Sprint(value)
	if strings.ContainsAny(text, " \t") {
		return strconv.Quote(text)
	}
	return text
}

func renderLlamaLog(lines []string) []string {
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, renderLlamaLine(line))
	}
	return rendered
}

// renderLlamaLine dims the "[pid] elapsed" prefix and colors the message by the
// lone I/W/E severity token, e.g. "[53069] 0.09.354 I srv llama_server: …".
func renderLlamaLine(line string) string {
	for _, token := range []string{" I ", " W ", " E "} {
		if cut := strings.Index(line, token); cut >= 0 {
			body := cut + 1 // start coloring at the severity char
			return ui.MutedStyle.Render(line[:body]) + severityStyle(token[1]).Render(line[body:])
		}
	}
	return ui.MutedStyle.Render(line)
}

func severityStyle(severity byte) lipgloss.Style {
	switch severity {
	case 'W':
		return warningStyle
	case 'E':
		return lipgloss.NewStyle().Foreground(ui.Red)
	default:
		return ui.GreenStyle
	}
}
