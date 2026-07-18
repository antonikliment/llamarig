package tabs

import (
	"fmt"
	"image/color"
	"llamarig/adapters/tui/ui"
	"math"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	controlv1 "llamarig/core/rpc/gen/v1"
)

var resourceStyle = lipgloss.NewStyle().Foreground(ui.Cyan)
var warningStyle = lipgloss.NewStyle().Foreground(ui.Yellow)

type SystemTab struct{ scroll int }

func (t *SystemTab) Update(msg tea.Msg, keys KeyMap) {
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(pressed, keys.Up) {
			t.scroll = max(0, t.scroll-1)
		}
		if key.Matches(pressed, keys.Down) {
			t.scroll++
		}
	}
}

func (t *SystemTab) View(width, height int, snapshot dashboardSnapshot) string {
	content := renderSystem(width, snapshot.resources, snapshot.warnings["resources"])
	t.scroll = min(t.scroll, max(0, strings.Count(content, "\n")+1-height))
	return ui.VerticalSlice(content, t.scroll, height)
}

func renderSystem(width int, resources *controlv1.RuntimeResources, warning string) string {
	// The footer already renders connection warnings, so the tab body shows
	// the resource panel alone instead of a second, redundant status line.
	return systemResourcesDetailPanel(width-4, 10, resources, warning)
}

func systemResourcesDetailPanel(width int, height int, resources *controlv1.RuntimeResources, warning string) string {
	content := []string{resourceStyle.Render("System Resources")}
	if warning != "" || resources == nil {
		content = append(content, warningStyle.Render("Warning: control socket not available"))
	} else {
		content = append(content,
			resourceRow("CPU", resources.GetCpuUsedPercent(), ""),
			resourceRow("RAM", resources.GetMemoryUsedPercent(), bytePair(resources.GetUsedRamBytes(), resources.GetTotalRamBytes())),
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

func resourceMeter(percent int) string {
	const segments = 20
	percent = max(0, min(100, percent))
	filled := percent * segments / 100
	bar := lipgloss.NewStyle().Foreground(meterColor(percent)).Render(strings.Repeat("█", filled))
	return bar + ui.MutedStyle.Render(strings.Repeat("░", segments-filled))
}

// meterColor shades a meter green/yellow/red by load so the fill conveys
// severity through shape and hue, not the cyan-only block of the old bars.
func meterColor(percent int) color.Color {
	switch {
	case percent >= 90:
		return ui.Red
	case percent >= 70:
		return ui.Yellow
	default:
		return ui.Green
	}
}

func bytePair(used uint64, total uint64) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("(%s/%s)", formatBytes(int64(used)), formatBytes(int64(total)))
}

func gpuRow(resources *controlv1.RuntimeResources) string {
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

func diskDetailRows(resources *controlv1.RuntimeResources) []string {
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
