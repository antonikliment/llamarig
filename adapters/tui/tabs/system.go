package tabs

import (
	"fmt"
	"llamarig/adapters/tui/ui"
	"math"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	controlv1 "llamarig/core/rpc/gen/v1"
)

var resourceStyle = lipgloss.NewStyle().Foreground(ui.Cyan)
var warningStyle = lipgloss.NewStyle().Foreground(ui.Yellow)

type SystemTab struct{ vp viewport.Model }

func NewSystemTab() SystemTab { return SystemTab{vp: viewport.New()} }

func (t *SystemTab) Update(msg tea.Msg, keys KeyMap) {
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(pressed, keys.Up) {
			t.vp.ScrollUp(1)
		}
		if key.Matches(pressed, keys.Down) {
			t.vp.ScrollDown(1)
		}
	}
}

func (t *SystemTab) View(width, height int, snapshot dashboardSnapshot) string {
	t.vp.SetWidth(width)
	t.vp.SetHeight(height)
	t.vp.SetContent(renderSystem(width, snapshot.resources, snapshot.warnings["resources"]))
	return t.vp.View()
}

func renderSystem(width int, resources *controlv1.SignalsSnapshot, warning string) string {
	// The footer already renders connection warnings, so the tab body shows
	// the resource panel alone instead of a second, redundant status line.
	return systemResourcesDetailPanel(width-4, 10, resources, warning)
}

func systemResourcesDetailPanel(width int, height int, resources *controlv1.SignalsSnapshot, warning string) string {
	content := []string{resourceStyle.Render("System Resources")}
	if warning != "" || resources == nil {
		content = append(content, warningStyle.Render("Warning: control socket not available"))
	} else {
		content = append(content,
			resourceRow("CPU", resources.GetCpu().GetUsedPercent(), ""),
			resourceRow("RAM", resources.GetMemory().GetUsedPercent(), bytePair(resources.GetMemory().GetUsedBytes(), resources.GetMemory().GetTotalBytes())),
			gpuRow(resources),
		)
		content = append(content, diskDetailRows(resources)...)
	}

	return ui.PanelStyle(ui.Cyan, false).
		Width(width).
		Height(height).
		Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}

func resourceRow(label string, percent float64, detail string) string {
	rounded := int(math.Round(percent))
	row := fmt.Sprintf("%-5s %s %3d%%", label+":", resourceMeter(rounded), rounded)
	if detail != "" {
		row += "  " + ui.MutedStyle.Render(detail)
	}

	return row
}

var meter = progress.New(progress.WithWidth(20), progress.WithFillCharacters('█', '░'), progress.WithoutPercentage(), progress.WithColors(ui.Green))

func resourceMeter(percent int) string {
	return meter.ViewAs(float64(max(0, min(100, percent))) / 100)
}

func bytePair(used uint64, total uint64) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("(%s/%s)", formatBytes(int64(used)), formatBytes(int64(total)))
}

func gpuRow(resources *controlv1.SignalsSnapshot) string {
	gpus := resources.GetGpu()
	if len(gpus) == 0 {
		detail := "(unavailable)"
		if warnings := resources.GetWarnings(); len(warnings) > 0 {
			detail = "(" + warnings[0] + ")"
		}
		return resourceRow("GPU", 0, detail)
	}
	gpu := gpus[0]
	detail := bytePair(gpu.GetUsedVramBytes(), gpu.GetTotalVramBytes())
	if gpu.GetName() != "" {
		detail = fmt.Sprintf("%s %s", detail, gpu.GetName())
	}
	return resourceRow("GPU", gpu.GetUtilizationPercent(), detail)
}

func diskDetailRows(resources *controlv1.SignalsSnapshot) []string {
	disks := resources.GetDisks()
	if len(disks) == 0 {
		return []string{resourceRow("Disk", 0, "(unavailable)")}
	}
	rows := make([]string, 0, len(disks))
	for _, disk := range disks {
		rows = append(rows, resourceRow(diskLabel(disk), disk.GetUsedPercent(), diskDetail(disk)))
	}
	return rows
}

func diskLabel(disk *controlv1.DiskSnapshot) string {
	switch disk.GetLabel() {
	case "model_storage":
		return "Models"
	case "root":
		return "Root"
	case "":
		return "Disk"
	default:
		return disk.GetLabel()
	}
}

func diskDetail(disk *controlv1.DiskSnapshot) string {
	detail := bytePair(disk.GetUsedBytes(), disk.GetTotalBytes())
	if path := disk.GetPath(); path != "" {
		detail = fmt.Sprintf("%s %s", detail, path)
	}
	return detail
}
