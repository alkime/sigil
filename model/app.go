package model

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
)

// AppState represents the current UI mode.
type AppState int

const (
	StateBrowse AppState = iota
	StateInspect
	StateHelp
)

// navState holds navigation state shared via pointer so that gutter/style
// closures see mutations even after AppModel is copied by bubbletea.
type navState struct {
	// rendered line index -> comment IDs covering that line
	renderedToComments map[int][]string

	// ordered list of rendered line indices that have comments (for n/N)
	commentLineIndices []int

	// focusedCommentIdx is the index into commentLineIndices (-1 = none)
	focusedCommentIdx int
}

// AppModel is the top-level Bubbletea model.
type AppModel struct {
	doc       *Document
	viewport  viewport.Model
	statusbar StatusBarModel
	modal     ModalModel
	state     AppState
	width     int
	height    int
	isDark    bool
	nav       *navState

	// rendered content as a single string
	renderedContent string
}

// NewApp creates a new AppModel from a parsed Document.
func NewApp(doc *Document) AppModel {
	m := AppModel{
		doc:    doc,
		state:  StateBrowse,
		isDark: true,
		nav:    &navState{focusedCommentIdx: -1},
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
		default:
			return m.updateBrowse(msg)
		}
	}

	// Pass other messages to viewport
	if m.state == StateBrowse {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.statusbar.scrollPct = m.viewport.ScrollPercent()
		return m, cmd
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

	case "?":
		m.state = StateHelp
		return m, nil
	}

	// Let viewport handle j/k/d/u/pgup/pgdn etc.
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

	borderColor := lipgloss.Color("#7D56F4")
	if !m.isDark {
		borderColor = lipgloss.Color("#9B72CF")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(modalWidth).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// initViewport renders the markdown and sets up the viewport with gutter and highlights.
func (m *AppModel) initViewport() {
	vpHeight := m.height - 1 // leave room for status bar

	// Render markdown with Glamour
	rendered := m.renderMarkdown()
	m.renderedContent = rendered

	// Build line mapping and comment associations
	renderedLines := strings.Split(rendered, "\n")
	m.buildRenderedCommentMap(renderedLines)

	// Set up viewport — disable j/k/up/down since we use them for comment navigation
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

// buildRenderedCommentMap builds the mapping from rendered line indices to comment IDs.
// Uses a best-effort heuristic: content line i maps roughly to rendered line i,
// adjusted by counting block boundaries.
func (m *AppModel) buildRenderedCommentMap(renderedLines []string) {
	m.nav.renderedToComments = make(map[int][]string)
	m.nav.commentLineIndices = nil

	if len(m.doc.CommentedContentLines) == 0 {
		return
	}

	// Build content-line to rendered-line mapping
	contentToRendered := buildLineMapping(m.doc.ContentLines, renderedLines)

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
	// This is what n/N cycles through.
	seen := make(map[string]bool)
	type commentTarget struct {
		id   string
		line int
	}
	var targets []commentTarget
	// Collect all rendered lines sorted, then pick first occurrence of each comment ID
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

// buildLineMapping maps content line indices to rendered line indices.
// Heuristic: align by blank-line-delimited blocks.
func buildLineMapping(contentLines []string, renderedLines []string) []int {
	mapping := make([]int, len(contentLines))

	if len(contentLines) == 0 || len(renderedLines) == 0 {
		return mapping
	}

	contentBlocks := identifyBlocks(contentLines)
	renderedBlocks := identifyBlocks(renderedLines)

	// Align blocks: for each content block, find the corresponding rendered block
	for ci := range contentLines {
		// Find which content block this line belongs to
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
			// Fallback: proportional mapping
			mapping[ci] = min(ci, len(renderedLines)-1)
			continue
		}

		rBlock := renderedBlocks[cBlockIdx]
		cBlockSize := cBlock.end - cBlock.start + 1
		rBlockSize := rBlock.end - rBlock.start + 1

		// Proportional position within the block
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
		isEmpty := strings.TrimSpace(line) == ""
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

	// Scroll so the focused comment is visible
	target := m.nav.commentLineIndices[m.nav.focusedCommentIdx]
	m.viewport.SetYOffset(target)
}

// tryOpenInspect opens the inspect modal for the focused comment, or the nearest one.
func (m *AppModel) tryOpenInspect() {
	var ids []string
	var ok bool

	// Prefer the focused comment
	if m.nav.focusedCommentIdx >= 0 && m.nav.focusedCommentIdx < len(m.nav.commentLineIndices) {
		ri := m.nav.commentLineIndices[m.nav.focusedCommentIdx]
		ids, ok = m.nav.renderedToComments[ri]
	}

	if !ok || len(ids) == 0 {
		// Fall back: check lines around current scroll position
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

	// Build source snippet from the comment's ref marker
	snippet := m.buildSnippet(comment, ids[0])

	m.modal = newInspectModal(comment, snippet, m.width, m.height)
	m.state = StateInspect
}

func (m *AppModel) buildSnippet(comment *ReviewComment, id string) string {
	// Find the ref marker for this comment
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

// Ensure AppModel satisfies tea.Model at compile time.
var _ tea.Model = AppModel{}
