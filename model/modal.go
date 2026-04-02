package model

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
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
	textarea       textarea.Model
	snippetVP      viewport.Model
	snippetFocused bool
	mode           ModalMode
	width          int
	height         int

	// For new comments
	selection SelectionResult

	// For editing existing comments
	commentID string
	status    string // "open" or "resolved"

	// Source line range (1-based, inclusive)
	startLine int
	endLine   int
}

func newCreateModal(snippet string, sel SelectionResult, startLine, endLine, width, height int) CommentModal {
	ta := makeTextarea(width)
	svp := makeSnippetViewport(snippet, width, height)
	return CommentModal{
		textarea:  ta,
		snippetVP: svp,
		mode:      ModalCreate,
		selection: sel,
		startLine: startLine,
		endLine:   endLine,
		width:     width,
		height:    height,
	}
}

func newEditModal(comment *ReviewComment, snippet string, startLine, endLine, width, height int) CommentModal {
	ta := makeTextarea(width)
	ta.SetValue(comment.Comment)
	svp := makeSnippetViewport(snippet, width, height)
	return CommentModal{
		textarea:  ta,
		snippetVP: svp,
		mode:      ModalEdit,
		commentID: comment.ID,
		status:    comment.Status,
		startLine: startLine,
		endLine:   endLine,
		width:     width,
		height:    height,
	}
}

// snippetViewportHeight calculates the available height for the snippet viewport.
// Fixed overhead: title(1) + blank(1) + separator(1) + blank(1) + textarea(5) + blank(1) + footer(1)
// + border/padding(4) = 15 lines, plus 3 lines safety margin.
func snippetViewportHeight(termHeight int) int {
	h := termHeight - 18
	return max(3, h)
}

func makeSnippetViewport(snippet string, width, height int) viewport.Model {
	modalWidth := min(width-4, 80)
	innerWidth := modalWidth - 6

	snippetLines := strings.Count(snippet, "\n") + 1
	vpHeight := snippetViewportHeight(height)
	if snippetLines < vpHeight {
		vpHeight = snippetLines
	}

	vp := viewport.New(viewport.WithWidth(innerWidth), viewport.WithHeight(vpHeight))
	// Disable all built-in keybindings — we handle scrolling manually
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.Down.SetEnabled(false)
	vp.KeyMap.PageUp.SetEnabled(false)
	vp.KeyMap.PageDown.SetEnabled(false)
	vp.KeyMap.HalfPageUp.SetEnabled(false)
	vp.KeyMap.HalfPageDown.SetEnabled(false)
	vp.SetContent(snippet)
	return vp
}

func (m *CommentModal) snippetScrollable() bool {
	return m.snippetVP.TotalLineCount() > m.snippetVP.Height()
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
	if m.snippetFocused {
		var cmd tea.Cmd
		m.snippetVP, cmd = m.snippetVP.Update(msg)
		return cmd
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return cmd
}

func (m *CommentModal) ToggleFocus() tea.Cmd {
	m.snippetFocused = !m.snippetFocused
	if m.snippetFocused {
		m.textarea.Blur()
		return nil
	}
	return m.textarea.Focus()
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

	lineRange := ""
	if m.startLine > 0 {
		if m.startLine == m.endLine {
			lineRange = fmt.Sprintf(" L%d", m.startLine)
		} else {
			lineRange = fmt.Sprintf(" L%d-%d", m.startLine, m.endLine)
		}
	}

	var title string
	switch m.mode {
	case ModalCreate:
		title = titleStyle.Foreground(lipgloss.Color("#FF8800")).Render("New Comment" + lineRange)
	case ModalEdit, ModalConfirmDelete:
		statusStyle := lipgloss.NewStyle().Bold(true)
		if m.status == "open" {
			statusStyle = statusStyle.Foreground(lipgloss.Color("#FF8800"))
		} else {
			statusStyle = statusStyle.Foreground(lipgloss.Color("#00CC66"))
		}
		title = fmt.Sprintf("Comment [%s]%s %s", m.commentID, lineRange, statusStyle.Render(m.status))
	}

	// Source snippet viewport
	snippetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))
	snippetBlock := snippetStyle.Render(m.snippetVP.View())

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

	tabHint := ""
	if m.snippetScrollable() {
		tabHint = "  [Tab] Snippet"
	}

	var footer string
	switch m.mode {
	case ModalCreate:
		footer = footerStyle.Render("[Ctrl+S] Submit  [Enter] Newline" + tabHint + "  [Esc] Cancel")
	case ModalEdit:
		footer = footerStyle.Render("[Ctrl+S] Save" + tabHint + "  [Esc] Cancel")
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
