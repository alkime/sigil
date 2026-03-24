package model

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ModalModel renders an inspect overlay showing a comment and its source context.
type ModalModel struct {
	comment *ReviewComment
	snippet string // source lines covered by the comment
	width   int
	height  int
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

	return renderModalBox(content, modalWidth, m.width, m.height, isDark)
}

// CommentModal is the create-comment modal with an embedded textarea.
type CommentModal struct {
	textarea  textarea.Model
	snippet   string
	selection SelectionResult
	width     int
	height    int
}

func newCommentModal(snippet string, sel SelectionResult, width, height int) CommentModal {
	ta := textarea.New()
	ta.SetWidth(min(width-4, 80) - 8) // inner width minus some padding
	ta.SetHeight(5)
	ta.Focus()
	// Rebind InsertNewline to Shift+Enter so Enter can submit
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("shift+enter"))

	return CommentModal{
		textarea:  ta,
		snippet:   snippet,
		selection: sel,
		width:     width,
		height:    height,
	}
}

// Update handles textarea input. Returns true if the user submitted (Enter).
func (m *CommentModal) Update(msg tea.Msg) (bool, tea.Cmd) {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return false, cmd
}

// Value returns the current textarea content.
func (m *CommentModal) Value() string {
	return m.textarea.Value()
}

// FocusCmd returns the command to focus the textarea (cursor blink etc).
func (m *CommentModal) FocusCmd() tea.Cmd {
	return m.textarea.Focus()
}

func (m CommentModal) View(isDark bool) string {
	modalWidth := min(m.width-4, 80)
	innerWidth := modalWidth - 6

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8800"))
	title := titleStyle.Render("New Comment")

	// Source snippet
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

	// Textarea
	taView := m.textarea.View()

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))
	footer := footerStyle.Render("[Enter] Submit  [Shift+Enter] Newline  [Esc] Cancel")

	content := strings.Join([]string{title, "", snippetBlock, sep, "", taView, "", footer}, "\n")

	return renderModalBox(content, modalWidth, m.width, m.height, isDark)
}

// renderModalBox renders content in a centered bordered box.
func renderModalBox(content string, modalWidth, totalWidth, totalHeight int, isDark bool) string {
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

	return lipgloss.Place(totalWidth, totalHeight, lipgloss.Center, lipgloss.Center, box)
}
