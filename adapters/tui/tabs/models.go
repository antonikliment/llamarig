package tabs

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"llamarig/adapters/tui/ui"
	controlv1 "llamarig/core/rpc/gen/v1"

	bindkey "charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/antonikliment/tuikit"
)

type ModelsTab struct {
	presetTable  table.Model
	modelTable   table.Model
	focusModels  bool
	presetStatus tuikit.Status
	modelStatus  tuikit.Status
	keys         KeyMap
}

type presetStartRequestMsg struct{ name string }
type presetStartResultMsg struct{ err error }

func NewModelsTab() ModelsTab {
	styles := table.DefaultStyles()
	styles.Selected = ui.SelectedRowStyle // teal highlight instead of the bubbles default pink
	return ModelsTab{
		keys: DefaultKeyMap(),
		presetTable: table.New(table.WithColumns([]table.Column{
			{Title: "Preset", Width: 16}, {Title: "Model", Width: 40}, {Title: "State", Width: 12},
		}), table.WithStyles(styles)),
		modelTable: table.New(table.WithColumns([]table.Column{
			{Title: "Local model", Width: 40}, {Title: "Size", Width: 10}, {Title: "Use", Width: 20},
		}), table.WithStyles(styles)),
	}
}

func (t *ModelsTab) setRows(snapshot dashboardSnapshot) {
	presetRows := make([]table.Row, 0, len(snapshot.presets))
	for i := range snapshot.presets {
		preset := &snapshot.presets[i]
		presetRows = append(presetRows, table.Row{preset.Name, presetModel(preset), presetState(preset, snapshot.runtime)})
	}
	t.presetTable.SetRows(presetRows)
	modelRows := make([]table.Row, 0, len(snapshot.localModels))
	for _, model := range snapshot.localModels {
		modelRows = append(modelRows, table.Row{model.GetFilename(), tuikit.FormatBytes(model.GetSizeBytes()), modelUse(model)})
	}
	t.modelTable.SetRows(modelRows)
}

func presetState(preset *presetView, runtime *controlv1.RuntimeStatus) string {
	switch {
	case presetUnavailable(preset):
		return "Unavailable"
	case presetRunning(runtime, preset.Name):
		return "Running"
	default:
		return "Stopped"
	}
}

func modelUse(model *controlv1.LocalModel) string {
	if len(model.GetUsedByPresets()) > 0 {
		return "in: " + strings.Join(model.GetUsedByPresets(), ",")
	}
	return ""
}

func (t *ModelsTab) Update(msg tea.Msg, snapshot dashboardSnapshot) tea.Cmd {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	t.setRows(snapshot)
	presets, models := snapshot.presets, snapshot.localModels
	if len(models) == 0 {
		t.focusModels = false
	} else if len(presets) == 0 {
		t.focusModels = true
	}
	if bindkey.Matches(key, t.keys.NextPanel) || bindkey.Matches(key, t.keys.PreviousPanel) {
		t.focusModels = !t.focusModels
		t.modelStatus.Disarm()
		return nil
	}
	if t.focusModels {
		return t.updateModels(key, models)
	}
	return t.updatePresets(key, snapshot)
}

func (t *ModelsTab) updatePresets(key tea.KeyPressMsg, snapshot dashboardSnapshot) tea.Cmd {
	presets := snapshot.presets
	if len(presets) == 0 {
		return nil
	}
	sel := min(t.presetTable.Cursor(), len(presets)-1)
	switch {
	case bindkey.Matches(key, t.keys.Up):
		t.presetTable.MoveUp(1)
		t.presetStatus.Clear()
	case bindkey.Matches(key, t.keys.Down):
		t.presetTable.MoveDown(1)
		t.presetStatus.Clear()
	case key.String() == "esc":
		t.presetStatus.Disarm()
	case (key.String() == "d" || key.String() == "y") && presetUnavailable(&presets[sel]):
		return t.cleanupSelected(&presets[sel], key.String() == "y")
	case bindkey.Matches(key, t.keys.RunAction) && !presetRunning(snapshot.runtime, presets[sel].Name):
		if presetUnavailable(&presets[sel]) {
			t.presetStatus.SetError(presets[sel].SourceError)
			return nil
		}
		name := presets[sel].Name
		return func() tea.Msg { return presetStartRequestMsg{name: name} }
	}
	return nil
}

func (t *ModelsTab) updateModels(key tea.KeyPressMsg, models []*controlv1.LocalModel) tea.Cmd {
	sel := min(t.modelTable.Cursor(), len(models)-1)
	switch {
	case bindkey.Matches(key, t.keys.Up):
		t.modelTable.MoveUp(1)
		t.modelStatus.Disarm()
	case bindkey.Matches(key, t.keys.Down):
		t.modelTable.MoveDown(1)
		t.modelStatus.Disarm()
	case key.String() == "esc":
		t.modelStatus.Disarm()
	case (key.String() == "d" || key.String() == "y") && len(models) > 0:
		return t.deleteSelected(models[sel], key.String() == "y")
	}
	return nil
}

func (t *ModelsTab) setResult(result presetStartResultMsg) {
	t.presetStatus.SetResult(result.err, "preset started")
}

func (t *ModelsTab) setDeleteResult(result modelDeleteResultMsg) {
	t.modelStatus.SetResult(result.err, "model deleted")
}

func (t *ModelsTab) setCleanupResult(result presetCleanupResultMsg) {
	t.presetStatus.SetResult(result.err, "preset cleaned up")
}

func (t *ModelsTab) cleanupSelected(preset *presetView, confirm bool) tea.Cmd {
	return t.presetStatus.Confirm(preset.Name, confirm, func() tea.Cmd {
		return func() tea.Msg { return presetCleanupRequestMsg{name: preset.Name} }
	})
}

func (t *ModelsTab) deleteSelected(model *controlv1.LocalModel, confirm bool) tea.Cmd {
	path := model.GetPath()
	if path == "" {
		return nil
	}
	return t.modelStatus.Confirm(path, confirm, func() tea.Cmd {
		return func() tea.Msg { return modelDeleteRequestMsg{path: path} }
	})
}

func (t *ModelsTab) View(width, height int, snapshot dashboardSnapshot) string {
	t.setRows(snapshot)
	presets, models := snapshot.presets, snapshot.localModels

	const helpHeight = 2
	detailH := max(3, height/3)
	tabbedH := max(6, height-detailH-helpHeight)
	tableH := max(1, tabbedH-5) // tab row (2) + notch line (1) + box bottom border (1) + status row (1)

	t.presetTable.SetWidth(width - 4)
	t.presetTable.SetHeight(tableH)
	t.modelTable.SetWidth(width - 4)
	t.modelTable.SetHeight(tableH)

	titles := []string{fmt.Sprintf("Presets (%d)", len(presets)), fmt.Sprintf("Local models (%d)", len(models))}
	accents := []color.Color{ui.Cyan, ui.Green}

	active := 0
	var body, detail string
	if t.focusModels {
		active = 1
		t.modelTable.Focus()
		t.presetTable.Blur()
		body = t.modelPane(snapshot)
		detail = localModelDetail(width, detailH, ui.Green, t.selectedModel(models))
	} else {
		t.presetTable.Focus()
		t.modelTable.Blur()
		body = t.presetPane(snapshot)
		detail = presetDetail(width, detailH, ui.Cyan, t.selectedPreset(presets), snapshot.runtime)
	}

	tabbed := ui.TabbedPanel(titles, accents, active, width, tabbedH, body)
	return ui.VerticalSlice(lipgloss.JoinVertical(lipgloss.Left, tabbed, detail, "", modelHelp(t.keys)), 0, height)
}

func (t *ModelsTab) presetPane(snapshot dashboardSnapshot) string {
	if len(snapshot.presets) == 0 {
		if warning := snapshot.warnings["presets"]; warning != "" {
			return warningStyle.Render("Presets unavailable: " + warning)
		}
		return ui.MutedStyle.Render("No model presets configured")
	}
	rows := []string{t.presetTable.View()}
	if pending := t.presetStatus.Pending(); pending != "" {
		rows = append(rows, warningStyle.Render("Cleanup "+pending+" and its default/autostart references? Press d or y to confirm, esc to cancel"))
	} else if !t.focusModels {
		rows = t.presetStatus.AppendRows(ui.Theme(), rows)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (t *ModelsTab) modelPane(snapshot dashboardSnapshot) string {
	if len(snapshot.localModels) == 0 {
		if warning := snapshot.warnings["models"]; warning != "" {
			return warningStyle.Render("Models unavailable: " + warning)
		}
		return ui.MutedStyle.Render("No local models downloaded")
	}
	rows := []string{t.modelTable.View()}
	if warning := t.pendingDeleteWarning(snapshot.localModels); warning != "" {
		rows = append(rows, warningStyle.Render(warning))
	} else {
		rows = t.modelStatus.AppendRows(ui.Theme(), rows)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (t *ModelsTab) selectedPreset(presets []presetView) *presetView {
	if len(presets) == 0 {
		return nil
	}
	return &presets[min(t.presetTable.Cursor(), len(presets)-1)]
}

func (t *ModelsTab) selectedModel(models []*controlv1.LocalModel) *controlv1.LocalModel {
	if len(models) == 0 {
		return nil
	}
	return models[min(t.modelTable.Cursor(), len(models)-1)]
}

func (t *ModelsTab) pendingDeleteWarning(models []*controlv1.LocalModel) string {
	for _, model := range models {
		if model.GetPath() == t.modelStatus.Pending() {
			if len(model.GetModelPathPresets()) > 0 {
				return "Delete model and Presets " + strings.Join(model.GetModelPathPresets(), ", ") + "? Press d or y to confirm, esc to cancel"
			}
			if len(model.GetModelsDirPresets()) > 0 {
				return "Model discovered by " + strings.Join(model.GetModelsDirPresets(), ", ") + "; directory Presets remain. Press d or y to confirm, esc to cancel"
			}
			return "Delete " + model.GetFilename() + "? Press d or y to confirm, esc to cancel"
		}
	}
	return ""
}

// presetDetail fills the space below the list with the selected preset so
// the tab is no longer mostly empty, surfacing the full model path and the
// action that applies to its current state.
func presetDetail(width, height int, accent color.Color, preset *presetView, runtime *controlv1.RuntimeStatus) string {
	if preset == nil {
		return ui.EmptyDetail(accent, width, height, "Select a preset to see details")
	}
	state, stateColor, action := "Stopped", ui.Muted, ui.GreenStyle.Render("Enter: start preset")
	if presetUnavailable(preset) {
		state, stateColor, action = "Unavailable", ui.Red, warningStyle.Render("d: cleanup preset")
	} else if presetRunning(runtime, preset.Name) {
		state, stateColor, action = "Running", ui.Green, ui.MutedStyle.Render("Running · stop from the Services tab")
	}
	rows := []string{
		ui.StatusTitle(preset.Name, state, accent, stateColor, width),
		ui.Field("Model", presetModel(preset)),
		ui.Field("Path", tuikit.TruncMiddle(preset.Model, width-12)),
	}
	if presetUnavailable(preset) {
		rows = append(rows, ui.Field("Reason", tuikit.TruncMiddle(preset.SourceError, width-12)))
	}
	if height >= 8 {
		rows = append(rows, ui.Rule(width), action)
	} else if height >= 7 {
		rows = append(rows, action)
	}
	return ui.PanelStyle(accent, false).Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func localModelDetail(width, height int, accent color.Color, model *controlv1.LocalModel) string {
	if model == nil {
		return ui.EmptyDetail(accent, width, height, "Select a local model to see details")
	}
	usedBy, stateColor := "-", ui.Muted
	if len(model.GetUsedByPresets()) > 0 {
		usedBy, stateColor = strings.Join(model.GetUsedByPresets(), ", "), ui.Yellow
	}
	rows := []string{
		ui.StatusTitle(model.GetFilename(), tuikit.FormatBytes(model.GetSizeBytes()), accent, stateColor, width),
		ui.Field("Path", tuikit.TruncMiddle(model.GetPath(), width-12)),
		ui.Field("Used by", tuikit.TruncMiddle(usedBy, width-12)),
	}
	if height >= 7 {
		rows = append(rows, ui.Rule(width), ui.MutedStyle.Render("d: delete model"))
	}
	return ui.PanelStyle(accent, false).Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func presetUnavailable(preset *presetView) bool {
	return preset != nil && preset.SourceStatus == "unavailable"
}

func presetModel(preset *presetView) string {
	if model := filepath.Base(preset.Model); model != "." && model != "" {
		return model
	}
	return "-"
}

func presetRunning(status *controlv1.RuntimeStatus, preset string) bool {
	for _, loaded := range status.GetPresets() {
		if loaded.GetName() == preset && loaded.GetState() == "running" {
			return true
		}
	}
	return false
}

func modelHelp(keys KeyMap) string {
	return tuikit.HelpLine(keys.Up, keys.RunAction, keys.NextPanel, keys.ServicesTab, keys.Refresh, keys.Quit)
}
