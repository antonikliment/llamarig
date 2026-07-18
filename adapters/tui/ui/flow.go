package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func Flow(width, gap int, blocks []string) string {
	if width <= 0 {
		return ""
	}
	gap = max(0, gap)
	spacer := strings.Repeat(" ", gap)
	var rows, row []string
	rowWidth := 0
	for _, block := range blocks {
		blockWidth := lipgloss.Width(block)
		added := blockWidth
		if len(row) > 0 {
			added += gap
		}
		if len(row) > 0 && rowWidth+added > width {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
			row, rowWidth = nil, 0
		}
		if len(row) > 0 && gap > 0 {
			row, rowWidth = append(row, spacer), rowWidth+gap
		}
		row, rowWidth = append(row, block), rowWidth+blockWidth
	}
	if len(row) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
