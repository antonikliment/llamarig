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
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/antonikliment/tuikit"
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
	focus logPane
	view  [paneCount]tuikit.SearchView
}

func NewLogsTab() LogsTab {
	t := LogsTab{}
	for i := range t.view {
		t.view[i] = tuikit.NewSearchView()
	}
	return t
}

func (t *LogsTab) IsSearching() bool { return t.view[t.focus].Searching() }

func (t *LogsTab) Update(msg tea.Msg, keys KeyMap) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	// Panel switching is the LogsTab's own concern; scrolling, search focus, and
	// clearing are delegated to the focused pane's SearchView. Only intercept
	// Tab/Shift+Tab while not searching, so those keys type into the query box.
	if !t.view[t.focus].Searching() {
		switch {
		case bindkey.Matches(key, keys.NextPanel):
			t.focus = (t.focus + 1) % paneCount
			return
		case bindkey.Matches(key, keys.PreviousPanel):
			t.focus = (t.focus + paneCount - 1) % paneCount
			return
		}
	}
	t.view[t.focus].Update(msg)
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

	// Feed each pane its full rendered log; the SearchView applies the live
	// query (matching visible text) and tracks scroll/follow itself.
	t.view[paneDaemon].SetLines(renderDaemonLog(snapshot.daemonLog))
	t.view[paneLlama].SetLines(renderLlamaLog(snapshot.llamaLog))

	titles := make([]string, paneCount)
	accents := make([]color.Color, paneCount)
	for pane := logPane(0); pane < paneCount; pane++ {
		titles[pane] = fmt.Sprintf("%s (%d)", logPaneMeta[pane].title, len(t.view[pane].Filtered()))
		accents[pane] = logPaneMeta[pane].accent
	}

	// SearchView owns the viewport sizing: tab row (2) + notch line (1) + box
	// bottom border (1) sit outside it, matching the TabbedPanel chrome.
	content := t.view[t.focus].View(width-4, max(1, tabbedH-4))

	tabbed := ui.TabbedPanel(titles, accents, int(t.focus), width, tabbedH, content)
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
	view := &t.view[t.focus]
	if view.Searching() {
		status = ui.MutedStyle.Render("Search: " + view.InputView() + "  (Enter/Esc to finish)")
	} else if query := view.Query(); query != "" {
		status = ui.MutedStyle.Render("Search: " + query + "  (Esc to clear)")
	}
	return ui.PanelStyle(ui.Muted, false).Width(width).Render(status)
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
