package tabs

import (
	"fmt"
	"path/filepath"
	"strings"

	"llamarig/adapters/tui/ui"
	controlv1 "llamarig/core/rpc/gen/v1"

	bindkey "charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ModelsTab struct {
	presetTable       table.Model
	modelTable        table.Model
	focusModels       bool
	message           string
	err               string
	pendingDeletePath string
	pendingCleanup    string
	keys              KeyMap
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
		modelRows = append(modelRows, table.Row{model.GetFilename(), formatBytes(model.GetSizeBytes()), modelUse(model)})
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
	if key.String() == "tab" && len(models) > 0 && len(presets) > 0 {
		t.focusModels, t.pendingDeletePath = !t.focusModels, ""
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
		t.pendingCleanup, t.message, t.err = "", "", ""
	case bindkey.Matches(key, t.keys.Down):
		t.presetTable.MoveDown(1)
		t.pendingCleanup, t.message, t.err = "", "", ""
	case key.String() == "esc":
		t.pendingCleanup = ""
	case (key.String() == "d" || key.String() == "y") && presetUnavailable(&presets[sel]):
		return t.cleanupSelected(&presets[sel], key.String() == "y")
	case bindkey.Matches(key, t.keys.RunAction) && !presetRunning(snapshot.runtime, presets[sel].Name):
		if presetUnavailable(&presets[sel]) {
			t.err = presets[sel].SourceError
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
		t.pendingDeletePath = ""
	case bindkey.Matches(key, t.keys.Down):
		t.modelTable.MoveDown(1)
		t.pendingDeletePath = ""
	case key.String() == "esc":
		t.pendingDeletePath = ""
	case (key.String() == "d" || key.String() == "y") && len(models) > 0:
		return t.deleteSelected(models[sel], key.String() == "y")
	}
	return nil
}

// applyResult sets *msg to okMsg and clears *errField, or clears *msg and
// records err.Error() in *errField.
func applyResult(err error, msg, errField *string, okMsg string) {
	*msg, *errField = okMsg, ""
	if err != nil {
		*msg, *errField = "", err.Error()
	}
}

func (t *ModelsTab) setResult(result presetStartResultMsg) {
	applyResult(result.err, &t.message, &t.err, "preset started")
}

func (t *ModelsTab) setDeleteResult(result modelDeleteResultMsg) {
	t.pendingDeletePath = ""
	applyResult(result.err, &t.message, &t.err, "model deleted")
}

func (t *ModelsTab) setCleanupResult(result presetCleanupResultMsg) {
	t.pendingCleanup = ""
	applyResult(result.err, &t.message, &t.err, "preset cleaned up")
}

// confirmPending arms *pending (and clears msg/errField) on the first
// press, and fires cmd once the same target is confirmed on a second press
// ("press again to confirm").
func confirmPending(pending, msg, errField *string, target string, confirm bool, fire func() tea.Cmd) tea.Cmd {
	if *pending != target {
		if confirm {
			return nil
		}
		*pending, *msg, *errField = target, "", ""
		return nil
	}
	*pending = ""
	return fire()
}

func (t *ModelsTab) cleanupSelected(preset *presetView, confirm bool) tea.Cmd {
	return confirmPending(&t.pendingCleanup, &t.message, &t.err, preset.Name, confirm, func() tea.Cmd {
		return func() tea.Msg { return presetCleanupRequestMsg{name: preset.Name} }
	})
}

func (t *ModelsTab) deleteSelected(model *controlv1.LocalModel, confirm bool) tea.Cmd {
	path := model.GetPath()
	if path == "" {
		return nil
	}
	return confirmPending(&t.pendingDeletePath, &t.message, &t.err, path, confirm, func() tea.Cmd {
		return func() tea.Msg { return modelDeleteRequestMsg{path: path} }
	})
}

func (t *ModelsTab) View(width, height int, snapshot dashboardSnapshot) string {
	t.setRows(snapshot)
	presets, models := snapshot.presets, snapshot.localModels
	presetH := max(5, height/3)
	modelH := max(5, height/3)
	detailH := max(3, height-presetH-modelH-2)

	if t.focusModels {
		t.modelTable.Focus()
		t.presetTable.Blur()
	} else {
		t.presetTable.Focus()
		t.modelTable.Blur()
	}
	t.presetTable.SetWidth(width - 2)
	t.presetTable.SetHeight(max(1, presetH-2))
	t.modelTable.SetWidth(width - 2)
	t.modelTable.SetHeight(max(1, modelH-2))

	presetList := ui.PanelStyle(ui.Cyan, !t.focusModels).Width(width).Height(presetH).Render(t.presetPane(snapshot))
	modelList := ui.PanelStyle(ui.Cyan, t.focusModels).Width(width).Height(modelH).Render(t.modelPane(snapshot))
	var detail string
	if t.focusModels {
		detail = localModelDetail(width, detailH, t.selectedModel(models))
	} else {
		detail = presetDetail(width, detailH, t.selectedPreset(presets), snapshot.runtime)
	}
	return ui.VerticalSlice(lipgloss.JoinVertical(lipgloss.Left, presetList, modelList, detail, "", modelHelp(t.keys)), 0, height)
}

func (t *ModelsTab) presetPane(snapshot dashboardSnapshot) string {
	if len(snapshot.presets) == 0 {
		if warning := snapshot.warnings["presets"]; warning != "" {
			return warningStyle.Render("Presets unavailable: " + warning)
		}
		return ui.MutedStyle.Render("No model presets configured")
	}
	rows := []string{t.presetTable.View()}
	if t.pendingCleanup != "" {
		rows = append(rows, warningStyle.Render("Cleanup "+t.pendingCleanup+" and its default/autostart references? Press d or y to confirm, esc to cancel"))
	} else if !t.focusModels {
		rows = appendStatusRows(rows, t.err, t.message)
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
		rows = appendStatusRows(rows, t.err, t.message)
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

// appendStatusRows appends a rendered error or (if no error) success row,
// if either is set.
func appendStatusRows(rows []string, err, message string) []string {
	if err != "" {
		return append(rows, warningStyle.Render(err))
	}
	if message != "" {
		return append(rows, ui.GreenStyle.Render(message))
	}
	return rows
}

func (t *ModelsTab) pendingDeleteWarning(models []*controlv1.LocalModel) string {
	for _, model := range models {
		if model.GetPath() == t.pendingDeletePath {
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
func presetDetail(width, height int, preset *presetView, runtime *controlv1.RuntimeStatus) string {
	if preset == nil {
		return ui.PanelStyle(ui.Cyan, false).Width(width).Height(height).Render(ui.MutedStyle.Render("Select a preset to see details"))
	}
	state, stateColor, action := "Stopped", ui.Muted, ui.GreenStyle.Render("Enter: start preset")
	if presetUnavailable(preset) {
		state, stateColor, action = "Unavailable", ui.Red, warningStyle.Render("d: cleanup preset")
	} else if presetRunning(runtime, preset.Name) {
		state, stateColor, action = "Running", ui.Green, ui.MutedStyle.Render("Running · stop from the Services tab")
	}
	rows := []string{
		ui.StatusTitle(preset.Name, state, ui.Cyan, stateColor, width),
		ui.Field("Model", presetModel(preset)),
		ui.Field("Path", truncMiddle(preset.Model, width-12)),
	}
	if presetUnavailable(preset) {
		rows = append(rows, ui.Field("Reason", truncMiddle(preset.SourceError, width-12)))
	}
	if height >= 8 {
		rows = append(rows, ui.Rule(width), action)
	} else if height >= 7 {
		rows = append(rows, action)
	}
	return ui.PanelStyle(ui.Cyan, false).Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func localModelDetail(width, height int, model *controlv1.LocalModel) string {
	if model == nil {
		return ui.PanelStyle(ui.Cyan, false).Width(width).Height(height).Render(ui.MutedStyle.Render("Select a local model to see details"))
	}
	usedBy, stateColor := "-", ui.Muted
	if len(model.GetUsedByPresets()) > 0 {
		usedBy, stateColor = strings.Join(model.GetUsedByPresets(), ", "), ui.Yellow
	}
	rows := []string{
		ui.StatusTitle(model.GetFilename(), formatBytes(model.GetSizeBytes()), ui.Cyan, stateColor, width),
		ui.Field("Path", truncMiddle(model.GetPath(), width-12)),
		ui.Field("Used by", truncMiddle(usedBy, width-12)),
	}
	if height >= 7 {
		rows = append(rows, ui.Rule(width), ui.MutedStyle.Render("d: delete model"))
	}
	return ui.PanelStyle(ui.Cyan, false).Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
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

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value, suffixes := float64(size), []string{"KiB", "MiB", "GiB", "TiB"}
	for _, suffix := range suffixes {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", value/unit)
}

func modelHelp(keys KeyMap) string {
	return helpLine(keys.Up, keys.RunAction, keys.NextPanel, keys.ServicesTab, keys.Refresh, keys.Quit)
}
