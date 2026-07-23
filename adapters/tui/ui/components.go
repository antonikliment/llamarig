package ui

import (
	"image/color"

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

// Theme exposes the shared tuikit palette so tabs can pass it to tuikit
// components (e.g. Status.AppendRows) that render in the app's colors.
func Theme() tuikit.Theme { return theme }

// EmptyDetail renders a muted "nothing selected" placeholder inside a panel.
func EmptyDetail(accent color.Color, width, height int, msg string) string {
	return theme.EmptyPanel(accent, width, height, msg)
}

// TabStrip renders labelled tabs; the active one is a folder tab in its accent
// color. Titles are pre-formatted by the caller (e.g. with counts).
func TabStrip(titles []string, accents []color.Color, active int) string {
	return theme.TabStrip(titles, accents, active)
}

// TabbedPanel renders the tab row joined seamlessly to a content panel (the
// active tab opens into it, no dividing line), both in the active accent.
func TabbedPanel(titles []string, accents []color.Color, active, width, height int, body string) string {
	return theme.TabbedPanel(titles, accents, active, width, height, body)
}

func StatusTitle(title, status string, titleColor, statusColor color.Color, width int) string {
	return theme.StatusTitle(title, status, titleColor, statusColor, width)
}

func Field(label, value string) string { return tuikit.Field(label, value) }

// Flow lays blocks out left-to-right, wrapping to a new row when the next block
// would overflow width, separated by gap spaces.
func Flow(width, gap int, blocks []string) string { return tuikit.Flow(width, gap, blocks) }

func Rule(width int) string { return theme.Rule(width) }

func VerticalSlice(content string, offset, height int) string {
	return tuikit.VerticalSlice(content, offset, height)
}

func ActionRow(foreground color.Color, selected int, labels []string, focused bool) string {
	return theme.ActionRow(foreground, selected, labels, focused)
}
