package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// StatusBarModel renders a bottom status bar with file info and comment counts.
type StatusBarModel struct {
	filename    string
	openCount   int
	totalCount  int
	scrollPct   float64
	width       int
}

func newStatusBar(filename string, comments []ReviewComment, width int) StatusBarModel {
	open := 0
	for _, c := range comments {
		if c.Status == "open" {
			open++
		}
	}
	return StatusBarModel{
		filename:   filename,
		openCount:  open,
		totalCount: len(comments),
		width:      width,
	}
}

func (m StatusBarModel) View(isDark bool) string {
	bg := lipgloss.Color("#7D56F4")
	fg := lipgloss.Color("#FFFFFF")
	if !isDark {
		bg = lipgloss.Color("#D7C4FF")
		fg = lipgloss.Color("#1A1A2E")
	}

	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Padding(0, 1)

	left := style.Render(m.filename)
	middle := style.Render(fmt.Sprintf("%d/%d comments open", m.openCount, m.totalCount))
	right := style.Render(fmt.Sprintf("%3.0f%%", m.scrollPct*100))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(middle) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	pad := lipgloss.NewStyle().
		Background(bg).
		Width(gap).
		Render("")

	return left + pad + middle + pad + right
}
