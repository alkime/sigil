package model

import (
	"fmt"
	"sort"
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
	StateSelect
	StateComment
)

// navState holds navigation state shared via pointer so that gutter/style
// closures see mutations even after AppModel is copied by bubbletea.
type navState struct {
	// rendered line index -> comment IDs covering that line
	renderedToComments map[int][]string

	// ordered list of rendered line indices that have comments (for j/k in browse)
	commentLineIndices []int

	// focusedCommentIdx is the index into commentLineIndices (-1 = none)
	focusedCommentIdx int

	// renderedToSource maps rendered line index -> 0-based source line in RawLines
	renderedToSource map[int]int

	// contentBlocks are the navigable blocks for select mode
	contentBlocks []ContentBlock

	// selector state (active during StateSelect)
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

	// rendered content as a single string
	renderedContent string
}

// NewApp creates a new AppModel from a parsed Document.
func NewApp(doc *Document, writeFn WriteFunc) AppModel {
	m := AppModel{
		doc:     doc,
		state:   StateBrowse,
		isDark:  true,
		nav:     &navState{focusedCommentIdx: -1},
		writeFn: writeFn,
	}
	return m
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
		case StateSelect:
			return m.updateSelect(msg)
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
		// Pass non-key messages (like cursor blink) to comment modal
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

	case "g":
		m.viewport.GotoTop()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "G":
		m.viewport.GotoBottom()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "j", "down":
		m.jumpToComment(1)
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "k", "up":
		m.jumpToComment(-1)
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "enter":
		m.tryOpenInspect()
		return m, nil

	case "a":
		m.enterSelectMode()
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

func (m AppModel) updateSelect(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.nav.selector.MoveDown()
		m.viewport.SetYOffset(m.nav.selector.CursorRenderedLine())
		return m, nil

	case "k", "up":
		m.nav.selector.MoveUp()
		m.viewport.SetYOffset(m.nav.selector.CursorRenderedLine())
		return m, nil

	case "enter":
		result := m.nav.selector.Confirm()
		if result != nil {
			m.openCommentModal(*result)
			return m, m.commentModal.FocusCmd()
		}
		return m, nil

	case "esc":
		m.exitSelectMode()
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
		m.exitSelectMode()
		return m, nil

	case "enter":
		// Submit the comment
		text := strings.TrimSpace(m.commentModal.Value())
		if text == "" {
			return m, nil
		}
		sel := m.commentModal.selection
		newDoc, err := m.writeFn(m.doc, sel.StartSourceLine, sel.Span, text)
		if err != nil {
			// TODO: show error in status bar
			m.commentModal = nil
			m.exitSelectMode()
			return m, nil
		}
		m.doc = newDoc
		m.commentModal = nil
		m.state = StateBrowse
		m.initViewport()
		return m, nil
	}

	// Pass to textarea for text input
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
		{"j / ↓", "Next comment"},
		{"k / ↑", "Previous comment"},
		{"Enter", "Inspect focused comment"},
		{"a", "Annotate (select lines)"},
		{"d", "Half-page down"},
		{"u", "Half-page up"},
		{"g", "Go to top"},
		{"G", "Go to bottom"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
	}

	modalWidth := min(m.width-4, 50)
	innerWidth := modalWidth - 6

	lines := []string{titleStyle.Render("Keybindings"), ""}
	for _, b := range bindings {
		key := keyStyle.Width(12).Render(b.key)
		desc := descStyle.Render(b.desc)
		lines = append(lines, key+desc)
	}
	lines = append(lines, "", descStyle.Render("[Esc/?/q] Close"))

	content := strings.Join(lines, "\n")
	_ = innerWidth

	return renderModalBox(content, modalWidth, m.width, m.height, m.isDark)
}

// enterSelectMode switches to block selection mode.
func (m *AppModel) enterSelectMode() {
	m.state = StateSelect
	m.nav.selector = NewBlockSelector(m.nav.contentBlocks)

	// Swap gutter and style for select mode
	m.viewport.LeftGutterFunc = m.selectGutterFunc
	m.viewport.StyleLineFunc = m.selectStyleFunc
}

// exitSelectMode returns to browse mode, restoring the original gutter.
func (m *AppModel) exitSelectMode() {
	m.state = StateBrowse
	m.viewport.LeftGutterFunc = m.gutterFunc
	m.viewport.StyleLineFunc = m.styleLineFunc
}

// openCommentModal transitions from select to comment creation.
func (m *AppModel) openCommentModal(sel SelectionResult) {
	// Build snippet from selected source lines
	startLine := sel.StartSourceLine
	endLine := sel.EndSourceLine + 1
	if endLine > len(m.doc.RawLines) {
		endLine = len(m.doc.RawLines)
	}
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

// initViewport renders the markdown and sets up the viewport with gutter and highlights.
func (m *AppModel) initViewport() {
	vpHeight := m.height - 1 // leave room for status bar

	// Render markdown with Glamour
	rendered := m.renderMarkdown()
	m.renderedContent = rendered

	// Build line mapping, comment associations, and content blocks
	renderedLines := strings.Split(rendered, "\n")
	m.buildRenderedCommentMap(renderedLines)
	m.buildContentBlocks(renderedLines)

	// Set up viewport — disable j/k/up/down since we use them for comment/select navigation
	m.viewport = viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(vpHeight))
	m.viewport.KeyMap.Up.SetEnabled(false)
	m.viewport.KeyMap.Down.SetEnabled(false)
	m.viewport.SetContent(rendered)

	// Configure gutter and line styling
	m.viewport.LeftGutterFunc = m.gutterFunc
	m.viewport.StyleLineFunc = m.styleLineFunc

	// Status bar
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
		glamour.WithWordWrap(m.width-4), // leave room for gutter
		glamour.WithStandardStyle(style),
	)
	if err != nil {
		return content // fallback to raw
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content
	}

	// Glamour adds trailing newlines; trim them
	return strings.TrimRight(rendered, "\n")
}

// buildRenderedCommentMap builds the mapping from rendered line indices to comment IDs,
// and also builds the renderedToSource reverse mapping.
func (m *AppModel) buildRenderedCommentMap(renderedLines []string) {
	m.nav.renderedToComments = make(map[int][]string)
	m.nav.commentLineIndices = nil
	m.nav.renderedToSource = make(map[int]int)

	// Build content-line to rendered-line mapping
	contentToRendered := buildLineMapping(m.doc.ContentLines, renderedLines)

	// Build reverse mapping: rendered -> content -> source
	for ci, ri := range contentToRendered {
		if ri < len(renderedLines) && ci < len(m.doc.ContentToSource) {
			m.nav.renderedToSource[ri] = m.doc.ContentToSource[ci]
		}
	}

	if len(m.doc.CommentedContentLines) == 0 {
		return
	}

	// Map commented content lines to rendered lines
	for ci, ids := range m.doc.CommentedContentLines {
		ri := ci // default: 1:1
		if ci < len(contentToRendered) {
			ri = contentToRendered[ci]
		}
		if ri < len(renderedLines) {
			m.nav.renderedToComments[ri] = ids
		}
	}

	// Build jump targets: one entry per distinct comment (first rendered line only).
	seen := make(map[string]bool)
	type commentTarget struct {
		id   string
		line int
	}
	var targets []commentTarget
	allLines := make([]int, 0, len(m.nav.renderedToComments))
	for ri := range m.nav.renderedToComments {
		allLines = append(allLines, ri)
	}
	sort.Ints(allLines)
	for _, ri := range allLines {
		for _, id := range m.nav.renderedToComments[ri] {
			if !seen[id] {
				seen[id] = true
				targets = append(targets, commentTarget{id, ri})
			}
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].line < targets[j].line })
	m.nav.commentLineIndices = make([]int, len(targets))
	for i, t := range targets {
		m.nav.commentLineIndices[i] = t.line
	}
}

// buildContentBlocks identifies navigable blocks in the rendered output
// and maps each to its source line range via renderedToSource.
func (m *AppModel) buildContentBlocks(renderedLines []string) {
	m.nav.contentBlocks = nil
	rBlocks := identifyBlocks(renderedLines)

	for _, rb := range rBlocks {
		// Find source line range for this rendered block
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
		// Fall back if no source mapping found
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

// buildLineMapping maps content line indices to rendered line indices.
// Heuristic: align by blank-line-delimited blocks.
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

// Browse mode gutter: shows comment markers
func (m *AppModel) gutterFunc(ctx viewport.GutterContext) string {
	if ctx.Soft || ctx.Index >= ctx.TotalLines {
		return "  "
	}

	if _, ok := m.nav.renderedToComments[ctx.Index]; ok {
		focused := m.nav.focusedCommentIdx >= 0 &&
			m.nav.focusedCommentIdx < len(m.nav.commentLineIndices) &&
			m.nav.commentLineIndices[m.nav.focusedCommentIdx] == ctx.Index
		if focused {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4400")).Bold(true)
			return style.Render("▶ ")
		}
		markerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800"))
		return markerStyle.Render("● ")
	}
	return "  "
}

func (m *AppModel) styleLineFunc(lineIdx int) lipgloss.Style {
	if m.nav.focusedCommentIdx >= 0 &&
		m.nav.focusedCommentIdx < len(m.nav.commentLineIndices) &&
		m.nav.commentLineIndices[m.nav.focusedCommentIdx] == lineIdx {
		return lipgloss.NewStyle().Background(lipgloss.Color("#333333"))
	}
	return lipgloss.NewStyle()
}

// Select mode gutter: shows block indicator
func (m *AppModel) selectGutterFunc(ctx viewport.GutterContext) string {
	if ctx.Index >= ctx.TotalLines {
		return "  "
	}
	if ctx.Soft {
		return "  "
	}

	isCursor := m.nav.selector.IsCursorBlock(ctx.Index)
	inSel := m.nav.selector.InSelection(ctx.Index)

	if isCursor {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800")).Bold(true)
		return style.Render("▸ ")
	}
	if inSel {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#BB8844"))
		return style.Render("│ ")
	}
	return "  "
}

// Select mode line styling: highlights cursor block and selected range
func (m *AppModel) selectStyleFunc(lineIdx int) lipgloss.Style {
	isCursor := m.nav.selector.IsCursorBlock(lineIdx)
	inSel := m.nav.selector.InSelection(lineIdx)

	if isCursor {
		return lipgloss.NewStyle().Background(lipgloss.Color("#2D4F7C"))
	}
	if inSel {
		return lipgloss.NewStyle().Background(lipgloss.Color("#1E3550"))
	}
	return lipgloss.NewStyle()
}

// jumpToComment cycles to the next (dir=1) or previous (dir=-1) comment.
func (m *AppModel) jumpToComment(dir int) {
	if len(m.nav.commentLineIndices) == 0 {
		return
	}

	if dir > 0 {
		m.nav.focusedCommentIdx = (m.nav.focusedCommentIdx + 1) % len(m.nav.commentLineIndices)
	} else {
		m.nav.focusedCommentIdx--
		if m.nav.focusedCommentIdx < 0 {
			m.nav.focusedCommentIdx = len(m.nav.commentLineIndices) - 1
		}
	}

	target := m.nav.commentLineIndices[m.nav.focusedCommentIdx]
	m.viewport.SetYOffset(target)
}

// tryOpenInspect opens the inspect modal for the focused comment, or the nearest one.
func (m *AppModel) tryOpenInspect() {
	var ids []string
	var ok bool

	if m.nav.focusedCommentIdx >= 0 && m.nav.focusedCommentIdx < len(m.nav.commentLineIndices) {
		ri := m.nav.commentLineIndices[m.nav.focusedCommentIdx]
		ids, ok = m.nav.renderedToComments[ri]
	}

	if !ok || len(ids) == 0 {
		currentLine := m.viewport.YOffset()
		for offset := 0; offset <= 5; offset++ {
			if ids, ok = m.nav.renderedToComments[currentLine+offset]; ok && len(ids) > 0 {
				break
			}
		}
		if !ok || len(ids) == 0 {
			return
		}
	}

	comment, ok := m.doc.CommentByID[ids[0]]
	if !ok {
		return
	}

	snippet := m.buildSnippet(comment, ids[0])
	m.modal = newInspectModal(comment, snippet, m.width, m.height)
	m.state = StateInspect
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

var _ tea.Model = AppModel{}
