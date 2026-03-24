package model

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ModalMode distinguishes new comment creation from editing an existing one.
type ModalMode int

const (
	ModalCreate ModalMode = iota
	ModalEdit
	ModalConfirmDelete
)

// CommentModal handles both creating new comments and editing existing ones.
type CommentModal struct {
	textarea  textarea.Model
	snippet   string
	mode      ModalMode
	width     int
	height    int

	// For new comments
	selection SelectionResult

	// For editing existing comments
	commentID string
	status    string // "open" or "resolved"
}

func newCreateModal(snippet string, sel SelectionResult, width, height int) CommentModal {
	ta := makeTextarea(width)
	return CommentModal{
		textarea:  ta,
		snippet:   snippet,
		mode:      ModalCreate,
		selection: sel,
		width:     width,
		height:    height,
	}
}

func newEditModal(comment *ReviewComment, snippet string, width, height int) CommentModal {
	ta := makeTextarea(width)
	ta.SetValue(comment.Comment)
	return CommentModal{
		textarea:  ta,
		snippet:   snippet,
		mode:      ModalEdit,
		commentID: comment.ID,
		status:    comment.Status,
		width:     width,
		height:    height,
	}
}

func makeTextarea(width int) textarea.Model {
	ta := textarea.New()
	ta.SetWidth(min(width-4, 80) - 8)
	ta.SetHeight(5)
	ta.Focus()
	// Enter inserts newlines (default). Submit is Ctrl+S, handled by parent.
	return ta
}

func (m *CommentModal) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return cmd
}

func (m *CommentModal) Value() string {
	return m.textarea.Value()
}

func (m *CommentModal) FocusCmd() tea.Cmd {
	return m.textarea.Focus()
}

func (m *CommentModal) SetConfirmDelete() {
	m.mode = ModalConfirmDelete
}

func (m *CommentModal) CancelConfirmDelete() {
	m.mode = ModalEdit
}

func (m CommentModal) View(isDark bool) string {
	modalWidth := min(m.width-4, 80)
	innerWidth := modalWidth - 6

	titleStyle := lipgloss.NewStyle().Bold(true)

	var title string
	switch m.mode {
	case ModalCreate:
		title = titleStyle.Foreground(lipgloss.Color("#FF8800")).Render("New Comment")
	case ModalEdit, ModalConfirmDelete:
		statusStyle := lipgloss.NewStyle().Bold(true)
		if m.status == "open" {
			statusStyle = statusStyle.Foreground(lipgloss.Color("#FF8800"))
		} else {
			statusStyle = statusStyle.Foreground(lipgloss.Color("#00CC66"))
		}
		title = fmt.Sprintf("Comment [%s] %s", m.commentID, statusStyle.Render(m.status))
	}

	// Source snippet
	snippetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(innerWidth)
	snippetLines := strings.Split(m.snippet, "\n")
	if len(snippetLines) > 6 {
		snippetLines = append(snippetLines[:5], "...")
	}
	snippetBlock := snippetStyle.Render(strings.Join(snippetLines, "\n"))

	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		Render(strings.Repeat("─", innerWidth))

	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	if m.mode == ModalConfirmDelete {
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF4444"))
		prompt := warnStyle.Render("Delete this comment permanently?")
		footer := footerStyle.Render("[y] Yes  [n/Esc] Cancel")
		content := strings.Join([]string{title, "", snippetBlock, sep, "", prompt, "", footer}, "\n")
		return renderModalBox(content, modalWidth, m.width, m.height, isDark)
	}

	taView := m.textarea.View()

	var footer string
	switch m.mode {
	case ModalCreate:
		footer = footerStyle.Render("[Ctrl+S] Submit  [Enter] Newline  [Esc] Cancel")
	case ModalEdit:
		footer = footerStyle.Render("[Ctrl+S] Save  [Esc] Cancel")
	}

	content := strings.Join([]string{title, "", snippetBlock, sep, "", taView, "", footer}, "\n")
	return renderModalBox(content, modalWidth, m.width, m.height, isDark)
}

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
