package model

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// WriteFunc is the signature for the comment-writing callback injected from main.
type WriteFunc func(doc *Document, sourceLine int, span int, comment string) (*Document, error)

// AppState represents the current UI mode.
type AppState int

const (
	StateBrowse AppState = iota
	StateInspect
	StateHelp
	StateComment
)

// navState holds navigation state shared via pointer so that gutter/style
// closures see mutations even after AppModel is copied by bubbletea.
type navState struct {
	// rendered line index -> comment IDs covering that line
	renderedToComments map[int][]string

	// renderedToSource maps rendered line index -> 0-based source line in RawLines
	renderedToSource map[int]int

	// contentBlocks are the navigable blocks
	contentBlocks []ContentBlock

	// commentedBlocks maps block index -> true if that block has a comment
	commentedBlocks map[int]bool

	// selector tracks the focused block
	selector SelectorState
}

// AppModel is the top-level Bubbletea model.
type AppModel struct {
	doc          *Document
	viewport     viewport.Model
	statusbar    StatusBarModel
	modal        ModalModel
	commentModal *CommentModal
	state        AppState
	width        int
	height       int
	isDark       bool
	nav          *navState
	writeFn      WriteFunc

	renderedContent string
}

// NewApp creates a new AppModel from a parsed Document.
func NewApp(doc *Document, writeFn WriteFunc) AppModel {
	return AppModel{
		doc:     doc,
		state:   StateBrowse,
		isDark:  true,
		nav:     &navState{},
		writeFn: writeFn,
	}
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.initViewport()
		return m, func() tea.Msg { return tea.RequestBackgroundColor() }

	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.initViewport()
		return m, nil

	case tea.KeyPressMsg:
		switch m.state {
		case StateInspect:
			return m.updateInspect(msg)
		case StateHelp:
			return m.updateHelp(msg)
		case StateComment:
			return m.updateComment(msg)
		default:
			return m.updateBrowse(msg)
		}
	}

	// Pass other messages based on state
	switch m.state {
	case StateBrowse:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, cmd
	case StateComment:
		if m.commentModal != nil {
			_, cmd := m.commentModal.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m AppModel) updateBrowse(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		m.nav.selector.MoveDown()
		m.viewport.SetYOffset(m.nav.selector.CursorRenderedLine())
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "k", "up":
		m.nav.selector.MoveUp()
		m.viewport.SetYOffset(m.nav.selector.CursorRenderedLine())
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "n":
		m.nav.selector.JumpToNextCommented(m.nav.commentedBlocks)
		m.viewport.SetYOffset(m.nav.selector.CursorRenderedLine())
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "N":
		m.nav.selector.JumpToPrevCommented(m.nav.commentedBlocks)
		m.viewport.SetYOffset(m.nav.selector.CursorRenderedLine())
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "enter":
		return m.handleEnter()

	case "g":
		m.viewport.GotoTop()
		m.nav.selector.CursorBlock = 0
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "G":
		m.viewport.GotoBottom()
		if len(m.nav.selector.Blocks) > 0 {
			m.nav.selector.CursorBlock = len(m.nav.selector.Blocks) - 1
		}
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "?":
		m.state = StateHelp
		return m, nil
	}

	// Let viewport handle d/u/pgup/pgdn etc.
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	m.statusbar.scrollPct = m.viewport.ScrollPercent()
	return m, cmd
}

// handleEnter: context-sensitive — inspect existing comment or create new one.
func (m AppModel) handleEnter() (tea.Model, tea.Cmd) {
	if m.nav.commentedBlocks[m.nav.selector.CursorBlock] {
		m.openInspectForBlock()
		return m, nil
	}
	// No comment on this block — create one
	result := m.nav.selector.Result()
	if result != nil {
		m.openCommentModal(*result)
		return m, m.commentModal.FocusCmd()
	}
	return m, nil
}

func (m AppModel) updateInspect(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = StateBrowse
		m.modal = ModalModel{}
		return m, nil
	}
	return m, nil
}

func (m AppModel) updateHelp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?":
		m.state = StateBrowse
		return m, nil
	}
	return m, nil
}

func (m AppModel) updateComment(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.commentModal == nil {
		m.state = StateBrowse
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.commentModal = nil
		m.state = StateBrowse
		return m, nil

	case "enter":
		text := strings.TrimSpace(m.commentModal.Value())
		if text == "" {
			return m, nil
		}
		sel := m.commentModal.selection
		newDoc, err := m.writeFn(m.doc, sel.StartSourceLine, sel.Span, text)
		if err != nil {
			m.commentModal = nil
			m.state = StateBrowse
			return m, nil
		}
		m.doc = newDoc
		m.commentModal = nil
		m.state = StateBrowse
		m.initViewport()
		return m, nil
	}

	_, cmd := m.commentModal.Update(msg)
	return m, cmd
}

func (m AppModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var content string
	switch m.state {
	case StateInspect:
		content = m.modal.View(m.isDark)
	case StateHelp:
		content = m.renderHelp()
	case StateComment:
		if m.commentModal != nil {
			content = m.commentModal.View(m.isDark)
		}
	default:
		vpView := m.viewport.View()
		sbView := m.statusbar.View(m.isDark)
		content = vpView + "\n" + sbView
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m AppModel) renderHelp() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8800"))
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	bindings := []struct{ key, desc string }{
		{"j / ↓", "Next block"},
		{"k / ↑", "Previous block"},
		{"n", "Next comment"},
		{"N", "Previous comment"},
		{"Enter", "Inspect or add comment"},
		{"d", "Half-page down"},
		{"u", "Half-page up"},
		{"g", "Go to top"},
		{"G", "Go to bottom"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
	}

	modalWidth := min(m.width-4, 50)

	lines := []string{titleStyle.Render("Keybindings"), ""}
	for _, b := range bindings {
		key := keyStyle.Width(12).Render(b.key)
		desc := descStyle.Render(b.desc)
		lines = append(lines, key+desc)
	}
	lines = append(lines, "", descStyle.Render("[Esc/?/q] Close"))

	content := strings.Join(lines, "\n")
	return renderModalBox(content, modalWidth, m.width, m.height, m.isDark)
}

// openCommentModal opens the comment creation modal for a block.
func (m *AppModel) openCommentModal(sel SelectionResult) {
	startLine := sel.StartSourceLine
	endLine := min(sel.EndSourceLine+1, len(m.doc.RawLines))
	lines := m.doc.RawLines[startLine:endLine]
	numbered := make([]string, len(lines))
	for i, l := range lines {
		numbered[i] = fmt.Sprintf("%d: %s", startLine+i+1, l)
	}
	snippet := strings.Join(numbered, "\n")

	cm := newCommentModal(snippet, sel, m.width, m.height)
	m.commentModal = &cm
	m.state = StateComment
}

// openInspectForBlock opens the inspect modal for the comment on the focused block.
func (m *AppModel) openInspectForBlock() {
	b := m.nav.selector.CurrentBlock()
	if b == nil {
		return
	}

	// Find comment IDs for any rendered line in this block
	for ri := b.RenderedStart; ri <= b.RenderedEnd; ri++ {
		ids, ok := m.nav.renderedToComments[ri]
		if !ok || len(ids) == 0 {
			continue
		}
		comment, ok := m.doc.CommentByID[ids[0]]
		if !ok {
			continue
		}
		snippet := m.buildSnippet(comment, ids[0])
		m.modal = newInspectModal(comment, snippet, m.width, m.height)
		m.state = StateInspect
		return
	}
}

// initViewport renders the markdown and sets up the viewport.
func (m *AppModel) initViewport() {
	vpHeight := m.height - 1

	rendered := m.renderMarkdown()
	m.renderedContent = rendered

	renderedLines := strings.Split(rendered, "\n")
	m.buildRenderedCommentMap(renderedLines)
	m.buildContentBlocks(renderedLines)
	m.buildCommentedBlocksMap()

	// Initialize selector with blocks
	m.nav.selector = NewBlockSelector(m.nav.contentBlocks)

	m.viewport = viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(vpHeight))
	m.viewport.KeyMap.Up.SetEnabled(false)
	m.viewport.KeyMap.Down.SetEnabled(false)
	m.viewport.SetContent(rendered)

	m.viewport.LeftGutterFunc = m.gutterFunc
	m.viewport.StyleLineFunc = m.styleLineFunc

	m.statusbar = newStatusBar(m.doc.FilePath, m.doc.Comments, m.width)
	m.statusbar.scrollPct = m.viewport.ScrollPercent()
}

func (m *AppModel) renderMarkdown() string {
	content := strings.Join(m.doc.ContentLines, "\n")
	if content == "" {
		return ""
	}

	style := "dark"
	if !m.isDark {
		style = "light"
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(m.width-4),
		glamour.WithStandardStyle(style),
	)
	if err != nil {
		return content
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content
	}

	return strings.TrimRight(rendered, "\n")
}

// buildRenderedCommentMap builds rendered line -> comment ID mapping.
func (m *AppModel) buildRenderedCommentMap(renderedLines []string) {
	m.nav.renderedToComments = make(map[int][]string)
	m.nav.renderedToSource = make(map[int]int)

	contentToRendered := buildLineMapping(m.doc.ContentLines, renderedLines)

	for ci, ri := range contentToRendered {
		if ri < len(renderedLines) && ci < len(m.doc.ContentToSource) {
			m.nav.renderedToSource[ri] = m.doc.ContentToSource[ci]
		}
	}

	for ci, ids := range m.doc.CommentedContentLines {
		ri := ci
		if ci < len(contentToRendered) {
			ri = contentToRendered[ci]
		}
		if ri < len(renderedLines) {
			m.nav.renderedToComments[ri] = ids
		}
	}
}

// buildContentBlocks identifies navigable blocks in the rendered output.
func (m *AppModel) buildContentBlocks(renderedLines []string) {
	m.nav.contentBlocks = nil
	rBlocks := identifyBlocks(renderedLines)

	for _, rb := range rBlocks {
		srcStart, srcEnd := -1, -1
		for ri := rb.start; ri <= rb.end; ri++ {
			if src, ok := m.nav.renderedToSource[ri]; ok {
				if srcStart < 0 || src < srcStart {
					srcStart = src
				}
				if src > srcEnd {
					srcEnd = src
				}
			}
		}
		if srcStart < 0 {
			srcStart = rb.start
			srcEnd = rb.end
		}

		m.nav.contentBlocks = append(m.nav.contentBlocks, ContentBlock{
			RenderedStart: rb.start,
			RenderedEnd:   rb.end,
			SourceStart:   srcStart,
			SourceEnd:     srcEnd,
		})
	}
}

// buildCommentedBlocksMap marks which blocks have comments.
func (m *AppModel) buildCommentedBlocksMap() {
	m.nav.commentedBlocks = make(map[int]bool)
	for bi, b := range m.nav.contentBlocks {
		for ri := b.RenderedStart; ri <= b.RenderedEnd; ri++ {
			if _, ok := m.nav.renderedToComments[ri]; ok {
				m.nav.commentedBlocks[bi] = true
				break
			}
		}
	}
}

// Unified gutter: ▸ on focused block, ● on commented blocks, space otherwise.
func (m *AppModel) gutterFunc(ctx viewport.GutterContext) string {
	if ctx.Soft || ctx.Index >= ctx.TotalLines {
		return "  "
	}

	isCursor := m.nav.selector.IsCursorBlock(ctx.Index)
	if isCursor {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800")).Bold(true)
		return style.Render("▸ ")
	}

	// Check if this line is in a commented block
	for ri := ctx.Index; ri >= 0; ri-- {
		if _, ok := m.nav.renderedToComments[ctx.Index]; ok {
			markerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800"))
			return markerStyle.Render("● ")
		}
		break
	}

	return "  "
}

// Unified style: highlight focused block.
func (m *AppModel) styleLineFunc(lineIdx int) lipgloss.Style {
	if m.nav.selector.IsCursorBlock(lineIdx) {
		return lipgloss.NewStyle().Background(lipgloss.Color("#2D4F7C"))
	}
	return lipgloss.NewStyle()
}

func (m *AppModel) buildSnippet(comment *ReviewComment, id string) string {
	for _, marker := range m.doc.RefMarkers {
		if marker.ID != id {
			continue
		}
		startLine := marker.SourceLine + comment.Offset
		endLine := startLine + comment.Span
		if endLine > len(m.doc.RawLines) {
			endLine = len(m.doc.RawLines)
		}
		if startLine >= len(m.doc.RawLines) {
			return ""
		}
		lines := m.doc.RawLines[startLine:endLine]
		numbered := make([]string, len(lines))
		for i, l := range lines {
			numbered[i] = fmt.Sprintf("%d: %s", startLine+i+1, l)
		}
		return strings.Join(numbered, "\n")
	}
	return ""
}

// Line mapping and block identification helpers.

func buildLineMapping(contentLines []string, renderedLines []string) []int {
	mapping := make([]int, len(contentLines))
	if len(contentLines) == 0 || len(renderedLines) == 0 {
		return mapping
	}

	contentBlocks := identifyBlocks(contentLines)
	renderedBlocks := identifyBlocks(renderedLines)

	for ci := range contentLines {
		cBlockIdx := -1
		var cBlock block
		lineInBlock := 0
		for bi, b := range contentBlocks {
			if ci >= b.start && ci <= b.end {
				cBlockIdx = bi
				cBlock = b
				lineInBlock = ci - b.start
				break
			}
		}

		if cBlockIdx < 0 || cBlockIdx >= len(renderedBlocks) {
			mapping[ci] = min(ci, len(renderedLines)-1)
			continue
		}

		rBlock := renderedBlocks[cBlockIdx]
		cBlockSize := cBlock.end - cBlock.start + 1
		rBlockSize := rBlock.end - rBlock.start + 1

		if cBlockSize > 0 {
			ri := rBlock.start + (lineInBlock * rBlockSize / cBlockSize)
			mapping[ci] = min(ri, len(renderedLines)-1)
		} else {
			mapping[ci] = rBlock.start
		}
	}

	return mapping
}

type block struct {
	start, end int
}

func identifyBlocks(lines []string) []block {
	var blocks []block
	inBlock := false
	var current block

	for i, line := range lines {
		isEmpty := strings.TrimSpace(ansi.Strip(line)) == ""
		if isEmpty {
			if inBlock {
				current.end = i - 1
				blocks = append(blocks, current)
				inBlock = false
			}
		} else {
			if !inBlock {
				current = block{start: i}
				inBlock = true
			}
		}
	}
	if inBlock {
		current.end = len(lines) - 1
		blocks = append(blocks, current)
	}

	return blocks
}

var _ tea.Model = AppModel{}
