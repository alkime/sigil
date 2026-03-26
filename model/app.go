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

// Callback types injected from main to avoid import cycles.
type WriteFunc func(doc *Document, sourceLine int, span int, comment string) (*Document, error)
type UpdateFunc func(doc *Document, id string, newText string, newStatus string) (*Document, error)
type DeleteFunc func(doc *Document, id string) (*Document, error)

// AppState represents the current UI mode.
type AppState int

const (
	StateBrowse AppState = iota
	StateInspect // edit modal for existing comment
	StateHelp
	StateComment // create modal for new comment
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
	commentModal *CommentModal
	state        AppState
	width        int
	height       int
	isDark       bool
	nav          *navState
	writeFn      WriteFunc
	updateFn     UpdateFunc
	deleteFn     DeleteFunc

	renderedContent string
}

// NewApp creates a new AppModel from a parsed Document.
func NewApp(doc *Document, writeFn WriteFunc, updateFn UpdateFunc, deleteFn DeleteFunc) AppModel {
	return AppModel{
		doc:      doc,
		state:    StateBrowse,
		isDark:   true,
		nav:      &navState{},
		writeFn:  writeFn,
		updateFn: updateFn,
		deleteFn: deleteFn,
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
	case StateComment, StateInspect:
		if m.commentModal != nil {
			cmd := m.commentModal.Update(msg)
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
		m.ensureBlockVisible()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "k", "up":
		m.nav.selector.MoveUp()
		m.ensureBlockVisible()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "n":
		m.nav.selector.JumpToNextCommented(m.nav.commentedBlocks)
		m.ensureBlockVisible()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "N":
		m.nav.selector.JumpToPrevCommented(m.nav.commentedBlocks)
		m.ensureBlockVisible()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "x":
		m.nav.selector.ToggleSelect()
		return m, nil

	case "esc":
		if m.nav.selector.Selecting {
			m.nav.selector.CancelSelect()
			return m, nil
		}
		return m, nil

	case "enter":
		return m.handleEnter()

	case "r":
		if m.nav.commentedBlocks[m.nav.selector.CursorBlock] {
			return m.handleResolve()
		}
		return m, nil

	case "d":
		if m.nav.commentedBlocks[m.nav.selector.CursorBlock] {
			return m.handleDelete()
		}
		return m, nil

	case "J", "shift+down":
		m.viewport.HalfPageDown()
		m.snapCursorToViewport()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

	case "K", "shift+up":
		m.viewport.HalfPageUp()
		m.snapCursorToViewport()
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, nil

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
	m.snapCursorToViewport()
	m.statusbar.scrollPct = m.viewport.ScrollPercent()
	return m, cmd
}

// handleResolve toggles open/resolved on the focused block's comment.
func (m AppModel) handleResolve() (tea.Model, tea.Cmd) {
	comment := m.focusedComment()
	if comment == nil {
		return m, nil
	}
	newStatus := "resolved"
	if comment.Status == "resolved" {
		newStatus = "open"
	}
	newDoc, err := m.updateFn(m.doc, comment.ID, comment.Comment, newStatus)
	if err == nil {
		m.doc = newDoc
		m.initViewport()
	}
	return m, nil
}

// handleDelete: two-stage — first resolves, then prompts for hard delete.
func (m AppModel) handleDelete() (tea.Model, tea.Cmd) {
	comment := m.focusedComment()
	if comment == nil {
		return m, nil
	}
	if comment.Status == "open" {
		// First d: soft delete (resolve)
		newDoc, err := m.updateFn(m.doc, comment.ID, comment.Comment, "resolved")
		if err == nil {
			m.doc = newDoc
			m.initViewport()
		}
		return m, nil
	}
	// Already resolved: open confirm-delete modal
	snippet := m.buildSnippetForComment(comment)
	cm := newEditModal(comment, snippet, m.width, m.height)
	cm.SetConfirmDelete()
	m.commentModal = &cm
	m.state = StateInspect
	return m, nil
}

// handleEnter: context-sensitive — edit existing comment or create new one.
func (m AppModel) handleEnter() (tea.Model, tea.Cmd) {
	// If multi-selecting, always create a new comment for the range
	if m.nav.selector.Selecting {
		result := m.nav.selector.Result()
		if result != nil {
			m.nav.selector.CancelSelect()
			m.openCommentModal(*result)
			return m, m.commentModal.FocusCmd()
		}
		return m, nil
	}

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
	if m.commentModal == nil {
		m.state = StateBrowse
		return m, nil
	}

	// Handle confirm-delete sub-state
	if m.commentModal.mode == ModalConfirmDelete {
		switch msg.String() {
		case "y":
			newDoc, err := m.deleteFn(m.doc, m.commentModal.commentID)
			if err == nil {
				m.doc = newDoc
			}
			m.commentModal = nil
			m.state = StateBrowse
			m.initViewport()
			return m, nil
		case "n", "esc":
			m.commentModal = nil
			m.state = StateBrowse
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.commentModal = nil
		m.state = StateBrowse
		return m, nil

	case "ctrl+s":
		text := strings.TrimSpace(m.commentModal.Value())
		if text != "" {
			newDoc, err := m.updateFn(m.doc, m.commentModal.commentID, text, m.commentModal.status)
			if err == nil {
				m.doc = newDoc
			}
		}
		m.commentModal = nil
		m.state = StateBrowse
		m.initViewport()
		return m, nil
	}

	// Pass to textarea
	cmd := m.commentModal.Update(msg)
	return m, cmd
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

	case "ctrl+s":
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

	cmd := m.commentModal.Update(msg)
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
	case StateInspect, StateComment:
		if m.commentModal != nil {
			content = m.commentModal.View(m.isDark)
		}
	case StateHelp:
		content = m.renderHelp()
	default:
		m.statusbar.onCommentBlock = m.nav.commentedBlocks[m.nav.selector.CursorBlock]
		vpView := m.viewport.View()
		contextHint := m.statusbar.ContextHintView()
		sbView := m.statusbar.View(m.isDark)
		content = vpView + "\n" + contextHint + "\n" + sbView
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
		{"n / N", "Next / prev comment"},
		{"x", "Select block range"},
		{"Enter", "Edit or add comment"},
		{"r", "Resolve / reopen"},
		{"d", "Delete comment"},
		{"Shift+j/k", "Half-page scroll"},
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
	snippet := m.buildSnippetRange(sel.StartSourceLine, sel.EndSourceLine+1)
	cm := newCreateModal(snippet, sel, m.width, m.height)
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
		cm := newEditModal(comment, snippet, m.width, m.height)
		m.commentModal = &cm
		m.state = StateInspect
		return
	}
}

// initViewport renders the markdown and sets up the viewport.
func (m *AppModel) initViewport() {
	vpHeight := m.height - 2 // status bar + context hint line

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
	m.viewport.KeyMap.HalfPageDown.SetEnabled(false)
	m.viewport.KeyMap.HalfPageUp.SetEnabled(false)
	m.viewport.SetContent(rendered)

	m.viewport.LeftGutterFunc = m.gutterFunc
	m.viewport.StyleLineFunc = m.styleLineFunc

	m.statusbar = newStatusBar(m.doc.FilePath, m.width)
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
	// Build individual blocks first
	var rawBlocks []ContentBlock
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

		rawBlocks = append(rawBlocks, ContentBlock{
			RenderedStart: rb.start,
			RenderedEnd:   rb.end,
			SourceStart:   srcStart,
			SourceEnd:     srcEnd,
		})
	}

	// Merge consecutive blocks that share the same comment ID into one super-block.
	m.nav.contentBlocks = nil
	for i := 0; i < len(rawBlocks); i++ {
		b := rawBlocks[i]
		commentID := m.blockCommentID(b)
		if commentID == "" {
			m.nav.contentBlocks = append(m.nav.contentBlocks, b)
			continue
		}
		// Merge subsequent blocks with the same comment ID
		merged := false
		for i+1 < len(rawBlocks) && m.blockCommentID(rawBlocks[i+1]) == commentID {
			i++
			b.RenderedEnd = rawBlocks[i].RenderedEnd
			b.SourceEnd = rawBlocks[i].SourceEnd
			merged = true
		}
		// Only extend to cover trailing blank lines if we actually merged multiple blocks
		if merged && i+1 < len(rawBlocks) {
			b.RenderedEnd = rawBlocks[i+1].RenderedStart - 1
		}
		m.nav.contentBlocks = append(m.nav.contentBlocks, b)
	}
}

// blockCommentID returns the comment ID for a block, or "" if uncommented.
func (m *AppModel) blockCommentID(b ContentBlock) string {
	for ri := b.RenderedStart; ri <= b.RenderedEnd; ri++ {
		if ids, ok := m.nav.renderedToComments[ri]; ok && len(ids) > 0 {
			return ids[0]
		}
	}
	return ""
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

// Unified gutter function.
// Focused + commented: ▶/│/▶ bracket in orange (open) or dim green (resolved)
// Focused + uncommented: blue ▎ bar
// Unfocused + commented: ● on first line in orange (open) or dim green (resolved)
// Unfocused + uncommented: blank
func (m *AppModel) gutterFunc(ctx viewport.GutterContext) string {
	isCursor := m.nav.selector.IsCursorBlock(ctx.Index)
	commentPos, resolved := m.commentLinePosition(ctx.Index)

	orangeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800"))
	orangeBold := orangeStyle.Bold(true)
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5A7A5A"))
	greenBold := greenStyle.Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	blueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5599FF"))

	// Pick color based on resolved status
	accentStyle := orangeStyle
	accentBold := orangeBold
	if resolved {
		accentStyle = greenStyle
		accentBold = greenBold
	}

	if ctx.Soft || ctx.Index >= ctx.TotalLines {
		if isCursor && commentPos != "" {
			return accentStyle.Render("│") + "  "
		}
		if isCursor {
			return "  " + blueStyle.Render("▎")
		}
		return "   "
	}

	// Focused commented block: bracket
	if isCursor && commentPos != "" {
		switch commentPos {
		case "only":
			return accentBold.Render("▶") + "  "
		case "first":
			return accentBold.Render("▶") + "  "
		case "middle":
			return accentStyle.Render("│") + "  "
		case "last":
			return accentBold.Render("▶") + "  "
		}
	}

	// Unfocused commented block: ● on first line, │ on middle, ╵ on last
	if commentPos != "" && !isCursor {
		switch commentPos {
		case "only":
			return accentStyle.Render("●") + "  "
		case "first":
			return accentStyle.Render("●") + "  "
		case "middle":
			return dimStyle.Render("│") + "  "
		case "last":
			return dimStyle.Render("╵") + "  "
		}
	}

	// Focused uncommented block: blue bar
	if isCursor {
		return "  " + blueStyle.Render("▎")
	}

	return "   "
}

// commentLinePosition returns where a rendered line sits in its commented block.
// Returns "first", "middle", "last", "only", or "" if not in a commented block.
func (m *AppModel) commentLinePosition(renderedLine int) (pos string, resolved bool) {
	// Find the commented block containing this line
	for bi, b := range m.nav.contentBlocks {
		if !m.nav.commentedBlocks[bi] {
			continue
		}
		if renderedLine < b.RenderedStart || renderedLine > b.RenderedEnd {
			continue
		}
		// Check if the comment is resolved
		for ri := b.RenderedStart; ri <= b.RenderedEnd; ri++ {
			if ids, ok := m.nav.renderedToComments[ri]; ok && len(ids) > 0 {
				if c, ok := m.doc.CommentByID[ids[0]]; ok {
					resolved = c.Status == "resolved"
				}
				break
			}
		}
		// This line is in a commented block
		if b.RenderedStart == b.RenderedEnd {
			return "only", resolved
		}
		if renderedLine == b.RenderedStart {
			return "first", resolved
		}
		if renderedLine == b.RenderedEnd {
			return "last", resolved
		}
		return "middle", resolved
	}
	return "", false
}

// firstLineOfCommentedBlock checks if renderedLine is the first line of the first
// commented block for its comment ID. Returns (blockIndex, isFirstLine).
// blockIndex is -1 if the line isn't in a commented block or isn't a content line.
func (m *AppModel) firstLineOfCommentedBlock(renderedLine int) (int, bool) {
	ids, ok := m.nav.renderedToComments[renderedLine]
	if !ok || len(ids) == 0 {
		return -1, false
	}
	if !m.isInContentBlock(renderedLine) {
		return -1, false
	}

	commentID := ids[0]

	// Find the first block that has this comment
	for bi, b := range m.nav.contentBlocks {
		if !m.nav.commentedBlocks[bi] {
			continue
		}
		for ri := b.RenderedStart; ri <= b.RenderedEnd; ri++ {
			if blockIDs, ok := m.nav.renderedToComments[ri]; ok && len(blockIDs) > 0 && blockIDs[0] == commentID {
				// This is the first block with this comment.
				// Return whether renderedLine is the first line of this block.
				return bi, renderedLine == b.RenderedStart
			}
		}
	}
	return -1, false
}

// isInContentBlock returns true if the rendered line is inside any content block.
func (m *AppModel) isInContentBlock(renderedLine int) bool {
	for _, b := range m.nav.contentBlocks {
		if renderedLine >= b.RenderedStart && renderedLine <= b.RenderedEnd {
			return true
		}
		if b.RenderedStart > renderedLine {
			break // blocks are sorted
		}
	}
	return false
}

func (m *AppModel) styleLineFunc(_ int) lipgloss.Style {
	return lipgloss.NewStyle()
}

// snapCursorToViewport moves the block cursor to the nearest visible block after a viewport scroll.
func (m *AppModel) snapCursorToViewport() {
	vpTop := m.viewport.YOffset()
	vpBottom := vpTop + m.viewport.Height() - 1

	// If current block is still visible, keep it
	b := m.nav.selector.CurrentBlock()
	if b != nil && b.RenderedEnd >= vpTop && b.RenderedStart <= vpBottom {
		return
	}

	// Find the first block that's visible
	for i, block := range m.nav.selector.Blocks {
		if block.RenderedEnd >= vpTop && block.RenderedStart <= vpBottom {
			m.nav.selector.CursorBlock = i
			return
		}
	}
}

// ensureBlockVisible scrolls the viewport only if the focused block is near the edge.
func (m *AppModel) ensureBlockVisible() {
	b := m.nav.selector.CurrentBlock()
	if b == nil {
		return
	}

	margin := 3 // lines of context to keep visible above/below
	vpTop := m.viewport.YOffset()
	vpBottom := vpTop + m.viewport.Height() - 1

	// If block start is above the viewport (with margin), scroll up
	if b.RenderedStart < vpTop+margin {
		m.viewport.SetYOffset(max(0, b.RenderedStart-margin))
	}

	// If block end is below the viewport (with margin), scroll down
	if b.RenderedEnd > vpBottom-margin {
		m.viewport.SetYOffset(b.RenderedEnd - m.viewport.Height() + 1 + margin)
	}
}

// focusedComment returns the comment on the currently focused block, or nil.
func (m *AppModel) focusedComment() *ReviewComment {
	b := m.nav.selector.CurrentBlock()
	if b == nil {
		return nil
	}
	for ri := b.RenderedStart; ri <= b.RenderedEnd; ri++ {
		ids, ok := m.nav.renderedToComments[ri]
		if !ok || len(ids) == 0 {
			continue
		}
		if c, ok := m.doc.CommentByID[ids[0]]; ok {
			return c
		}
	}
	return nil
}

// buildSnippetForComment builds a numbered source snippet for any comment.
func (m *AppModel) buildSnippetForComment(comment *ReviewComment) string {
	return m.buildSnippet(comment, comment.ID)
}

func (m *AppModel) buildSnippet(comment *ReviewComment, id string) string {
	for _, marker := range m.doc.RefMarkers {
		if marker.ID != id {
			continue
		}
		startLine := marker.SourceLine + comment.Offset
		endLine := startLine + comment.Span
		return m.buildSnippetRange(startLine, endLine)
	}
	return ""
}

// buildSnippetRange builds a numbered source snippet with context lines above/below.
// startLine and endLine are 0-based indices in RawLines (endLine exclusive).
func (m *AppModel) buildSnippetRange(startLine, endLine int) string {
	contextLines := 2

	ctxStart := max(0, startLine-contextLines)
	ctxEnd := min(len(m.doc.RawLines), endLine+contextLines)

	if ctxStart >= len(m.doc.RawLines) {
		return ""
	}

	maxNum := ctxEnd
	numWidth := len(fmt.Sprintf("%d", maxNum))

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	var numbered []string
	for i := ctxStart; i < ctxEnd; i++ {
		l := m.doc.RawLines[i]
		// Skip ref marker lines
		if strings.Contains(l, "@review-ref") {
			continue
		}
		numStr := fmt.Sprintf("%*d", numWidth, i+1)
		if i < startLine || i >= endLine {
			numbered = append(numbered, dimStyle.Render(fmt.Sprintf("%s: %s", numStr, l)))
		} else {
			numbered = append(numbered, fmt.Sprintf("%s: %s", numStr, l))
		}
	}
	return strings.Join(numbered, "\n")
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
