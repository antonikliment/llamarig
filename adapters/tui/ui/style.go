package ui

import "charm.land/lipgloss/v2"

var (
	Green  = lipgloss.Color("10")
	Blue   = lipgloss.Color("12")
	Yellow = lipgloss.Color("11")
	Red    = lipgloss.Color("9")
	Cyan   = lipgloss.Color("14")
	Muted  = lipgloss.Color("8")

	AppStyle         = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Muted).Padding(0, 1)
	HeaderStyle      = lipgloss.NewStyle()
	FooterStyle      = lipgloss.NewStyle()
	BrandStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	MutedStyle       = lipgloss.NewStyle().Foreground(Muted)
	SubtleStyle      = lipgloss.NewStyle().Foreground(Muted).Faint(true)
	GreenStyle       = lipgloss.NewStyle().Foreground(Green)
	ActiveTabStyle   = lipgloss.NewStyle().Foreground(Blue).Bold(true).Underline(true)
	InactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	SelectedRowStyle = lipgloss.NewStyle().Background(Cyan).Foreground(lipgloss.Color("0"))
)
