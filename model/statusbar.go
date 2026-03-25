package model

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// StatusBarModel renders the bottom status area: keybinding hints + file info.
type StatusBarModel struct {
	filename       string
	scrollPct      float64
	width          int
	onCommentBlock bool
}

func newStatusBar(filename string, width int) StatusBarModel {
	return StatusBarModel{
		filename: filename,
		width:    width,
	}
}

// ContextHintView renders the contextual hint line.
// Always returns a string (blank when no comment block focused) to keep layout stable.
func (m StatusBarModel) ContextHintView() string {
	if !m.onCommentBlock {
		return ""
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	hints := hintStyle.Render("r: resolve/reopen   d: delete")
	pad := strings.Repeat(" ", max(0, m.width-lipgloss.Width(hints)))

	return pad + hints
}

func (m StatusBarModel) View(isDark bool) string {
	bg := lipgloss.Color("#7D56F4")
	fg := lipgloss.Color("#FFFFFF")
	dimFg := lipgloss.Color("#C4B5E3")
	if !isDark {
		bg = lipgloss.Color("#D7C4FF")
		fg = lipgloss.Color("#1A1A2E")
		dimFg = lipgloss.Color("#6B5B8A")
	}

	infoStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Padding(0, 1)

	hintStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(dimFg).
		Padding(0, 1)

	left := infoStyle.Render(m.filename)
	hints := hintStyle.Render("j/k: blocks   n/N: comments   Enter: edit   ?: help")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(hints)
	if gap < 0 {
		gap = 0
	}

	pad := lipgloss.NewStyle().
		Background(bg).
		Width(gap).
		Render("")

	return left + pad + hints
}
