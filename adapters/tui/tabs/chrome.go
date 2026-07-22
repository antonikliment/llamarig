package tabs

import (
	"llamarig/adapters/tui/ui"
	"llamarig/config"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

type ChromeProps struct {
	ActiveTab Tab
	Width     int
	Warning   string
	Notice    string
	Refreshed time.Time
}

func RenderHeader(props ChromeProps) string {
	title := ui.BrandStyle.Render(config.ProjectDisplayName) + ui.MutedStyle.Render("  Local AI control service")
	labels := []struct {
		tab        Tab
		key, label string
	}{
		{TabServices, "1", "Services"}, {TabModels, "2", "Models"},
		{TabSystem, "3", "System"}, {TabLogs, "4", "Logs"},
	}
	parts := make([]string, 0, len(labels))
	for _, l := range labels {
		parts = append(parts, tabLabel(props.ActiveTab, l.tab, l.key, l.label))
	}
	tabLabels := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(parts, "    "))
	help := ui.MutedStyle.Render("Ctrl+C Quit")
	middleWidth := max(1, props.Width-lipgloss.Width(title)-lipgloss.Width(help))
	line := title + lipgloss.PlaceHorizontal(middleWidth, lipgloss.Center, tabLabels) + help

	return ui.HeaderStyle.Width(props.Width).Render(line)
}

func RenderFooter(props ChromeProps) string {
	status := ui.SubtleStyle.Render("Status: Ready")
	switch {
	case props.Warning != "":
		status = warningStyle.Render("Status: " + props.Warning)
	case props.Notice != "":
		status = ui.GreenStyle.Render("Status: " + props.Notice)
	}
	refreshed := "--:--:--"
	if !props.Refreshed.IsZero() {
		refreshed = props.Refreshed.Format("15:04:05")
	}
	right := ui.SubtleStyle.Render("Last refreshed: " + refreshed)
	space := max(1, props.Width-lipgloss.Width(status)-lipgloss.Width(right))

	return ui.FooterStyle.Width(props.Width).Render(status + strings.Repeat(" ", space) + right)
}

func tabLabel(activeTab Tab, tab Tab, key string, label string) string {
	text := "[" + key + "] " + label
	if activeTab == tab {
		return ui.ActiveTabStyle.Render(text)
	}
	return ui.InactiveTabStyle.Render(text)
}
