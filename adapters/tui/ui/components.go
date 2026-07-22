package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

func PanelStyle(color color.Color, focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(color).Padding(0, 1)
	if focused {
		return style.Border(lipgloss.DoubleBorder()).BorderForeground(Yellow)
	}
	return style
}

var inactiveTabChip = lipgloss.NewStyle().Foreground(Muted).Padding(0, 1)

func activeTabChip(accent color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(accent).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
}

// TabStrip renders labelled tab chips, highlighting the active one as a filled
// accent chip and the rest as muted labels. Titles are pre-formatted by the
// caller (e.g. with counts). len(titles) must equal len(accents).
func TabStrip(titles []string, accents []color.Color, active int) string {
	chips := make([]string, len(titles))
	for i, title := range titles {
		if i == active {
			chips[i] = activeTabChip(accents[i]).Render(title)
		} else {
			chips[i] = inactiveTabChip.Render(title)
		}
	}
	return strings.Join(chips, " ")
}

func StatusTitle(title, status string, titleColor, statusColor color.Color, width int) string {
	left := lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render(title)
	right := lipgloss.NewStyle().Foreground(statusColor).Render("● " + status)
	space := max(1, max(0, width-8)-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", space) + right
}

func Field(label, value string) string { return fmt.Sprintf("%-9s %s", label+":", value) }

func ActionRow(foreground color.Color, selected int, labels []string, focused bool) string {
	if !focused {
		return MutedStyle.Render("Actions:  " + strings.Join(labels, "  "))
	}
	parts := []string{lipgloss.NewStyle().Foreground(foreground).Render("Actions:")}
	for index, label := range labels {
		if index == selected {
			parts = append(parts, ActiveTabStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, MutedStyle.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

func Rule(width int) string { return MutedStyle.Render(strings.Repeat("─", max(0, width-6))) }

func VerticalSlice(content string, offset, height int) string {
	lines := strings.Split(content, "\n")
	if height <= 0 || len(lines) <= height {
		return content
	}
	offset = min(max(0, offset), len(lines)-height)
	return strings.Join(lines[offset:offset+height], "\n")
}
