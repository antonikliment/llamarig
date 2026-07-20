package tabs

import (
	"fmt"
	"path/filepath"
	"strings"

	"llamarig/adapters/tui/ui"
	controlv1 "llamarig/core/rpc/gen/v1"

	bindkey "charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ModelsTab struct {
	selected          int // preset selection
	presetOffset      int // preset list scroll
	modelSel          int // local model selection
	offset            int // model list scroll
	focusModels       bool
	message           string
	err               string
	pendingDeletePath string
	pendingCleanup    string
	keys              KeyMap
}

type presetStartRequestMsg struct{ name string }
type presetStartResultMsg struct{ err error }

func NewModelsTab() ModelsTab { return ModelsTab{keys: DefaultKeyMap()} }

func (t *ModelsTab) Update(msg tea.Msg, snapshot dashboardSnapshot) tea.Cmd {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	presets, models := snapshot.presets, snapshot.localModels
	if len(presets) > 0 {
		t.selected = min(t.selected, len(presets)-1)
	}
	if len(models) == 0 {
		t.focusModels = false
	} else {
		t.modelSel = min(t.modelSel, len(models)-1)
		if len(presets) == 0 {
			t.focusModels = true
		}
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
	switch {
	case bindkey.Matches(key, t.keys.Up):
		t.selected, t.pendingCleanup, t.message, t.err = max(0, t.selected-1), "", "", ""
	case bindkey.Matches(key, t.keys.Down):
		t.selected, t.pendingCleanup, t.message, t.err = min(len(presets)-1, t.selected+1), "", "", ""
	case key.String() == "esc":
		t.pendingCleanup = ""
	case (key.String() == "d" || key.String() == "y") && presetUnavailable(&presets[t.selected]):
		return t.cleanupSelected(&presets[t.selected], key.String() == "y")
	case bindkey.Matches(key, t.keys.RunAction) && !presetRunning(snapshot.runtime, presets[t.selected].Name):
		if presetUnavailable(&presets[t.selected]) {
			t.err = presets[t.selected].SourceError
			return nil
		}
		name := presets[t.selected].Name
		return func() tea.Msg { return presetStartRequestMsg{name: name} }
	}
	return nil
}

func (t *ModelsTab) updateModels(key tea.KeyPressMsg, models []*controlv1.LocalModel) tea.Cmd {
	switch {
	case bindkey.Matches(key, t.keys.Up):
		t.modelSel, t.pendingDeletePath = max(0, t.modelSel-1), ""
	case bindkey.Matches(key, t.keys.Down):
		t.modelSel, t.pendingDeletePath = min(len(models)-1, t.modelSel+1), ""
	case key.String() == "esc":
		t.pendingDeletePath = ""
	case (key.String() == "d" || key.String() == "y") && len(models) > 0:
		return t.deleteSelected(models[t.modelSel], key.String() == "y")
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
	presets, models := snapshot.presets, snapshot.localModels
	if len(presets) > 0 {
		t.selected = min(t.selected, len(presets)-1)
	}
	if t.modelSel >= len(models) {
		t.modelSel = max(0, len(models)-1)
	}
	presetH := max(5, height/3)
	modelH := max(5, height/3)
	detailH := max(3, height-presetH-modelH-2)
	t.presetOffset = clampOffset(t.selected, t.presetOffset, max(1, presetH-3))
	t.offset = clampOffset(t.modelSel, t.offset, max(1, modelH-4))
	presetList := ui.PanelStyle(ui.Purple, !t.focusModels).Width(width).Height(presetH).
		Render(lipgloss.JoinVertical(lipgloss.Left, t.presetRows(width, max(1, presetH-3), snapshot)...))
	modelList := ui.PanelStyle(ui.Purple, t.focusModels).Width(width).Height(modelH).
		Render(lipgloss.JoinVertical(lipgloss.Left, t.modelRows(width, max(1, modelH-4), snapshot)...))
	var detail string
	if t.focusModels {
		detail = localModelDetail(width, detailH, t.selectedModel(models))
	} else {
		detail = presetDetail(width, detailH, t.selectedPreset(presets), snapshot.runtime)
	}
	return ui.VerticalSlice(lipgloss.JoinVertical(lipgloss.Left, presetList, modelList, detail, "", modelHelp(t.keys)), 0, height)
}

// clampOffset keeps sel within [offset, offset+visible) by adjusting offset.
func clampOffset(sel, offset, visible int) int {
	if sel < offset {
		return sel
	}
	if sel >= offset+visible {
		return sel - visible + 1
	}
	return offset
}

func (t *ModelsTab) selectedPreset(presets []presetView) *presetView {
	if len(presets) == 0 {
		return nil
	}
	return &presets[t.selected]
}

func (t *ModelsTab) selectedModel(models []*controlv1.LocalModel) *controlv1.LocalModel {
	if len(models) == 0 {
		return nil
	}
	return models[t.modelSel]
}

func (t *ModelsTab) presetRows(width, visible int, snapshot dashboardSnapshot) []string {
	presets := snapshot.presets
	rows := []string{presetsHeader()}
	switch {
	case len(presets) == 0 && snapshot.warnings["presets"] != "":
		rows = append(rows, warningStyle.Render("Presets unavailable: "+snapshot.warnings["presets"]))
	case len(presets) == 0:
		rows = append(rows, ui.MutedStyle.Render("No model presets configured"))
	default:
		for index := t.presetOffset; index < min(len(presets), t.presetOffset+visible); index++ {
			rows = append(rows, presetRow(&presets[index], snapshot.runtime, index == t.selected && !t.focusModels, width))
		}
	}
	if t.pendingCleanup != "" {
		rows = append(rows, warningStyle.Render("Cleanup "+t.pendingCleanup+" and its default/autostart references? Press d or y to confirm, esc to cancel"))
	} else if !t.focusModels {
		rows = appendStatusRows(rows, t.err, t.message)
	}
	return rows
}

func (t *ModelsTab) modelRows(width, visible int, snapshot dashboardSnapshot) []string {
	models := snapshot.localModels
	rows := []string{modelsHeader()}
	switch {
	case len(models) == 0 && snapshot.warnings["models"] != "":
		rows = append(rows, warningStyle.Render("Models unavailable: "+snapshot.warnings["models"]))
	case len(models) == 0:
		rows = append(rows, ui.MutedStyle.Render("No local models downloaded"))
	default:
		for index := t.offset; index < min(len(models), t.offset+visible); index++ {
			rows = append(rows, localModelRow(models[index], index == t.modelSel && t.focusModels, width))
		}
	}
	if warning := t.pendingDeleteWarning(models); warning != "" {
		rows = append(rows, warningStyle.Render(warning))
	} else {
		rows = appendStatusRows(rows, t.err, t.message)
	}
	return rows
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

func presetsHeader() string { return fmt.Sprintf("    %-16s %-40s %s", "Preset", "Model", "State") }

func modelsHeader() string { return fmt.Sprintf("    %-40s %-10s %s", "Local model", "Size", "Use") }

// truncateRunes clips text to limit runes if it's longer and limit is positive.
func truncateRunes(text string, limit int) string {
	if runed := []rune(text); limit > 0 && len(runed) > limit {
		return string(runed[:limit])
	}
	return text
}

// selectRow applies the selected-row style, if selected.
func selectRow(row string, selected bool, width int) string {
	if selected {
		return ui.SelectedRowStyle.Width(width).Render(row)
	}
	return row
}

func rowMarker(selected bool) string {
	if selected {
		return ">"
	}
	return " "
}

func localModelRow(model *controlv1.LocalModel, selected bool, width int) string {
	used := ""
	if len(model.GetUsedByPresets()) > 0 {
		used = "in: " + strings.Join(model.GetUsedByPresets(), ",")
	}
	text := fmt.Sprintf("%-40.40s %-10s %s", model.GetFilename(), formatBytes(model.GetSizeBytes()), used)
	text = truncateRunes(text, width-4)
	return selectRow(rowMarker(selected)+" "+text, selected, width)
}

func presetRow(preset *presetView, runtime *controlv1.RuntimeStatus, selected bool, width int) string {
	dot, dotStyle := "○", ui.MutedStyle
	state := "Stopped"
	if presetUnavailable(preset) {
		dot, dotStyle, state = "!", warningStyle, "Unavailable"
	} else if presetRunning(runtime, preset.Name) {
		dot, dotStyle, state = "●", ui.GreenStyle, "Running"
	}
	text := fmt.Sprintf("%-16.16s %-40.40s %s", preset.Name, presetModel(preset), state)
	text = truncateRunes(text, width-6)
	if !selected {
		dot = dotStyle.Render(dot)
	}
	return selectRow(rowMarker(selected)+" "+dot+" "+text, selected, width)
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
	return ui.MutedStyle.Render("Quick Help  " + helpText(keys.Up) + "  " + helpText(keys.RunAction) + "  Tab Focus  d Delete  " + helpText(keys.ServicesTab) + "  " + helpText(keys.Refresh) + "  " + helpText(keys.Quit))
}
