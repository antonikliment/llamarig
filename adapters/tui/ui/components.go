package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/antonikliment/tuikit"
)

// theme mirrors the ui palette so the shared tuikit components render with
// llamarig's colors.
var theme = tuikit.Theme{
	Green:       Green,
	Blue:        Blue,
	Yellow:      Yellow,
	Red:         Red,
	Cyan:        Cyan,
	Muted:       Muted,
	Brand:       lipgloss.Color("63"),
	TabActiveFg: lipgloss.Color("0"),
	FocusBorder: Yellow,
}

func PanelStyle(c color.Color, focused bool) lipgloss.Style { return theme.PanelStyle(c, focused) }

// TabStrip renders labelled tabs; the active one is a folder tab in its accent
// color. Titles are pre-formatted by the caller (e.g. with counts).
func TabStrip(titles []string, accents []color.Color, active int) string {
	return theme.TabStrip(titles, accents, active)
}

func StatusTitle(title, status string, titleColor, statusColor color.Color, width int) string {
	return theme.StatusTitle(title, status, titleColor, statusColor, width)
}

func Field(label, value string) string { return tuikit.Field(label, value) }

func Rule(width int) string { return theme.Rule(width) }

func VerticalSlice(content string, offset, height int) string {
	return tuikit.VerticalSlice(content, offset, height)
}

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
