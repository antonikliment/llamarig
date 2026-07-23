package ui

import "charm.land/lipgloss/v2"

var (
	Green  = lipgloss.Color("10")
	Blue   = lipgloss.Color("12")
	Yellow = lipgloss.Color("11")
	Red    = lipgloss.Color("9")
	Cyan   = lipgloss.Color("14")
	Muted  = lipgloss.Color("8")

	BrandStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	MutedStyle       = lipgloss.NewStyle().Foreground(Muted)
	GreenStyle       = lipgloss.NewStyle().Foreground(Green)
	SelectedRowStyle = lipgloss.NewStyle().Background(Cyan).Foreground(lipgloss.Color("0"))
)
