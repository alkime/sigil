package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// ModalModel renders an inspect overlay showing a comment and its source context.
type ModalModel struct {
	comment   *ReviewComment
	snippet   string // source lines covered by the comment
	width     int
	height    int
}

func newInspectModal(comment *ReviewComment, snippet string, width, height int) ModalModel {
	return ModalModel{
		comment: comment,
		snippet: snippet,
		width:   width,
		height:  height,
	}
}

func (m ModalModel) View(isDark bool) string {
	if m.comment == nil {
		return ""
	}

	modalWidth := min(m.width-4, 80)
	innerWidth := modalWidth - 6 // border (2) + padding (4)

	// Status badge
	statusStyle := lipgloss.NewStyle().Bold(true)
	if m.comment.Status == "open" {
		statusStyle = statusStyle.Foreground(lipgloss.Color("#FF8800"))
	} else {
		statusStyle = statusStyle.Foreground(lipgloss.Color("#00CC66"))
	}
	title := fmt.Sprintf("Review Comment [%s] %s", m.comment.ID, statusStyle.Render(m.comment.Status))

	// Source snippet (dimmed, truncated)
	snippetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(innerWidth)
	snippetLines := strings.Split(m.snippet, "\n")
	if len(snippetLines) > 6 {
		snippetLines = append(snippetLines[:5], "...")
	}
	snippetBlock := snippetStyle.Render(strings.Join(snippetLines, "\n"))

	// Separator
	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		Render(strings.Repeat("─", innerWidth))

	// Comment text
	commentStyle := lipgloss.NewStyle().Width(innerWidth)
	commentBlock := commentStyle.Render(m.comment.Comment)

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))
	footer := footerStyle.Render("[Esc] Close")

	content := strings.Join([]string{title, "", snippetBlock, sep, commentBlock, "", footer}, "\n")

	borderColor := lipgloss.Color("#7D56F4")
	if !isDark {
		borderColor = lipgloss.Color("#9B72CF")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(modalWidth).
		Render(content)

	// Center the box in the available space
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
