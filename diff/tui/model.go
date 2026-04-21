package tui

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alkime/sigil/diff"
	"github.com/alkime/sigil/lsp"
)

type clearStatusMsg struct{}

func clearStatusCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)
		return clearStatusMsg{}
	}
}

// Mode represents the current UI mode.
type Mode int

const (
	ModeNormal     Mode = iota
	ModeComment         // textarea overlay for comment entry
	ModeInspect         // read-only modal for an existing comment
	ModeOrphan          // reading/resolving an orphaned comment
	ModeHelp            // keybinding help overlay
	ModePicker          // multi-PR picker (handled by pickerModel, not Model)
	ModeDefinition      // full-file read-only viewer for LSP go-to-definition targets
)

// Model is the Bubbletea model for the diff TUI.
// All fields are value types (no shared state pointers) so that
// each Update() call returns a correct independent copy.
type Model struct {
	session  *diff.Session
	pd       *diff.ParsedDiff
	files    []diff.ParsedFile
	fileIdx  int
	viewport viewport.Model
	keymap   KeyMap
	orphans  []string
	orphanIdx int
	comments []*diff.Comment
	mode     Mode
	statusMsg string
	width    int
	height   int

	// Navigation metadata (rebuilt on file change / window resize)
	focusedLine      int
	focusedCol       int
	linesMeta        []lineInfo
	hunkStarts       []int
	commentPositions []commentPos
	renderedLines    []string
	fileNumWidth     int
	fileExt          string

	// ModeComment state
	commentTA      textarea.Model
	commentFileIdx int
	commentHunkIdx int
	commentSide    diff.Side

	// ModeInspect state
	inspectID       string
	inspectTA       textarea.Model
	inspectHunkVP   viewport.Model
	inspectHunkFocus bool // true = hunk viewport focused, false = textarea focused

	// ModeOrphan state
	orphanVP viewport.Model

	// storage
	org      string
	repoName string

	// worktreePath is the absolute filesystem path of the worktree backing
	// session.Branch. Empty when no matching worktree is available; LSP and
	// other path-dependent features must degrade gracefully in that case.
	worktreePath string

	// LSP state — populated lazily on the first successful gd.
	lspManager   *lsp.Manager
	lspReqCancel context.CancelFunc

	// lastKey records the most recent normal-mode key, used for chord
	// detection (e.g. 'gd' → go-to-definition). Cleared on every non-chord key.
	lastKey string

	// jumpHistory is a bounded stack of prior locations for ctrl+o.
	jumpHistory []jumpEntry

	// lineIndex maps file.NewPath → NewLineNum → rendered line idx within
	// the currently focused file; rebuilt on every rebuildDiffView.
	lineIndex map[string]map[int32]int

	// ModeDefinition state — a read-only full-file viewer shown when a
	// go-to-definition target is outside the current diff (see definition.go).
	defFile       string
	defLine       int
	defSymbol     string
	defVP         viewport.Model
	defLines      []string
	defMeta       []lineInfo // reserved for future recursive-gd column data
	defIsExternal bool
}

// New creates a new diff TUI Model, loading and orphan-marking comments.
// worktreePath is the absolute path of the worktree matching session.Branch
// (empty when unavailable). When set, the model emits a stale-HEAD warning
// at startup if the worktree's HEAD diverges from session.HeadSHA.
func New(session *diff.Session, pd *diff.ParsedDiff, worktreePath string) Model {
	org, repoName := splitRepoLocal(session.Repo)

	loaded, _ := diff.LoadComments(org, repoName, session.PRNumber)
	comments := make([]*diff.Comment, len(loaded))
	for i := range loaded {
		c := loaded[i]
		comments[i] = &c
	}

	orphanedIDs := diff.MarkOrphans(comments, pd)

	m := Model{
		session:      session,
		pd:           pd,
		files:        pd.Files,
		keymap:       DefaultKeyMap(),
		comments:     comments,
		orphans:      orphanedIDs,
		org:          org,
		repoName:     repoName,
		worktreePath: worktreePath,
	}

	if len(orphanedIDs) > 0 {
		m.statusMsg = fmt.Sprintf("%d orphaned comment(s) — press o to review", len(orphanedIDs))
	} else if msg := staleHeadWarning(worktreePath, session); msg != "" {
		m.statusMsg = msg
	}

	return m
}

// staleHeadWarning returns a status message when the worktree's HEAD differs
// from session.HeadSHA. Returns empty string when worktreePath is empty, when
// git is unavailable, or when HEADs match.
func staleHeadWarning(worktreePath string, session *diff.Session) string {
	if worktreePath == "" || session == nil || session.HeadSHA == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	headSHA := strings.TrimSpace(string(out))
	if headSHA == "" || headSHA == session.HeadSHA {
		return ""
	}
	return "LSP results may be stale: worktree HEAD differs from session snapshot"
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case defResultMsg:
		cmd := m.handleDefResult(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildDiffView()
		if m.mode == ModeOrphan {
			m.loadOrphanView()
		}
		return m, nil

	case tea.KeyPressMsg:
		switch m.mode {
		case ModeHelp:
			return m.updateHelp(msg)
		case ModeComment:
			return m.updateComment(msg)
		case ModeInspect:
			return m.updateInspect(msg)
		case ModeOrphan:
			return m.updateOrphan(msg)
		case ModeDefinition:
			return updateDefinition(&m, msg)
		default:
			return m.updateNormal(msg)
		}
	}

	// Pass non-key messages to the active component.
	switch m.mode {
	case ModeOrphan:
		var cmd tea.Cmd
		m.orphanVP, cmd = m.orphanVP.Update(msg)
		return m, cmd
	case ModeComment:
		var cmd tea.Cmd
		m.commentTA, cmd = m.commentTA.Update(msg)
		return m, cmd
	case ModeInspect:
		var cmd tea.Cmd
		if m.inspectHunkFocus {
			m.inspectHunkVP, cmd = m.inspectHunkVP.Update(msg)
		} else {
			m.inspectTA, cmd = m.inspectTA.Update(msg)
		}
		return m, cmd
	case ModeDefinition:
		var cmd tea.Cmd
		m.defVP, cmd = m.defVP.Update(msg)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

func (m Model) updateNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keyStr := msg.String()

	// 'gd' chord: 'g' records lastKey; subsequent 'd' fires go-to-definition.
	if keyStr == "g" {
		m.lastKey = "g"
		return m, nil
	}
	if keyStr == "d" && m.lastKey == "g" {
		m.lastKey = ""
		cmd := m.goToDefinition()
		return m, cmd
	}
	// Any other key clears the chord state.
	m.lastKey = ""

	fl := m.focusedLine
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		m.moveLine(1)
		m.focusedCol = 0
		return m, nil

	case "k", "up":
		m.moveLine(-1)
		m.focusedCol = 0
		return m, nil

	case "J":
		m.jumpHunk(1)
		m.focusedCol = 0
		return m, nil

	case "K":
		m.jumpHunk(-1)
		m.focusedCol = 0
		return m, nil

	case "tab":
		m.cycleFile(1)
		m.focusedCol = 0
		return m, nil

	case "shift+tab":
		m.cycleFile(-1)
		m.focusedCol = 0
		return m, nil

	case "n":
		m.jumpComment(1)
		m.focusedCol = 0
		return m, nil

	case "N":
		m.jumpComment(-1)
		m.focusedCol = 0
		return m, nil

	case "h":
		m.moveCursor(-1)
		return m, nil

	case "l":
		m.moveCursor(1)
		return m, nil

	case "w":
		m.cursorWord(WordNext)
		return m, nil

	case "b":
		m.cursorWord(WordPrev)
		return m, nil

	case "e":
		m.cursorWord(WordEnd)
		return m, nil

	case "enter":
		if fl < len(m.linesMeta) && m.linesMeta[fl].isComment {
			return m.enterInspectMode(m.linesMeta[fl].commentID)
		}
		if m.fileIdx == -1 {
			return m.enterPRCommentMode()
		}
		if len(m.files) == 0 || !m.files[m.fileIdx].IsCommentable() {
			return m, nil
		}
		return m.enterCommentMode()

	case "c":
		if m.fileIdx == -1 {
			// In the PR Comments view, lowercase c also opens a PR-level comment.
			return m.enterPRCommentMode()
		}
		if len(m.files) == 0 {
			return m, nil
		}
		if !m.files[m.fileIdx].IsCommentable() {
			m.statusMsg = "comments not available for this file type"
			return m, nil
		}
		return m.enterCommentMode()

	case "r":
		if fl < len(m.linesMeta) && m.linesMeta[fl].isComment {
			return m, m.toggleCommentResolved(m.linesMeta[fl].commentID, true)
		}
		return m, nil

	case "u":
		if fl < len(m.linesMeta) && m.linesMeta[fl].isComment {
			return m, m.toggleCommentResolved(m.linesMeta[fl].commentID, false)
		}
		return m, nil

	case "o":
		if len(m.orphans) == 0 {
			m.statusMsg = "no orphaned comments"
			return m, nil
		}
		return m.enterOrphanMode()

	case "ctrl+]":
		cmd := m.goToDefinition()
		return m, cmd

	case "ctrl+o":
		if !m.popJump() {
			m.statusMsg = "no previous location"
		}
		return m, nil

	case "?":
		m.mode = ModeHelp
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) updateHelp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?":
		m.mode = ModeNormal
	}
	return m, nil
}

func (m Model) updateComment(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		m.statusMsg = ""
		return m, nil

	case "ctrl+s":
		body := strings.TrimSpace(m.commentTA.Value())
		if body == "" {
			m.mode = ModeNormal
			m.statusMsg = ""
			return m, nil
		}
		if err := m.submitComment(body); err != nil {
			m.statusMsg = fmt.Sprintf("error saving comment: %v", err)
			return m, nil
		}
		m.statusMsg = "comment added"
		m.mode = ModeNormal
		return m, clearStatusCmd()
	}

	var cmd tea.Cmd
	m.commentTA, cmd = m.commentTA.Update(msg)
	return m, cmd
}

func (m Model) enterInspectMode(id string) (tea.Model, tea.Cmd) {
	c := m.findComment(id)
	if c == nil {
		return m, nil
	}

	modalW := min(m.width-4, 80)
	innerW := modalW - 6 // subtract padding + border

	// Build hunk viewport.
	lines := m.hunkLines(c)
	hunkContent := hunkStyle.Render(c.HunkHeader) + "\n" + strings.Join(lines, "\n")
	vpH := min(len(lines)+1, 20)
	if vpH < 5 {
		vpH = 5
	}
	vp := viewport.New(viewport.WithWidth(innerW), viewport.WithHeight(vpH))
	vp.SetContent(hunkContent)

	// Build textarea.
	ta := textarea.New()
	ta.SetWidth(innerW)
	ta.SetHeight(6)
	ta.SetValue(c.Body)

	m.inspectID = id
	m.inspectTA = ta
	m.inspectHunkVP = vp
	m.inspectHunkFocus = false
	m.mode = ModeInspect
	return m, m.inspectTA.Focus()
}

func (m Model) updateInspect(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		m.inspectID = ""
		return m, nil

	case "ctrl+s":
		body := strings.TrimSpace(m.inspectTA.Value())
		for _, c := range m.comments {
			if c.ID == m.inspectID {
				if body != "" {
					c.Body = body
					c.UpdatedAt = time.Now().UTC()
				}
				break
			}
		}
		if err := m.saveComments(); err != nil {
			m.statusMsg = fmt.Sprintf("error saving: %v", err)
			return m, nil
		}
		m.statusMsg = "comment updated"
		m.mode = ModeNormal
		m.inspectID = ""
		m.rebuildDiffView()
		return m, clearStatusCmd()

	case "tab":
		m.inspectHunkFocus = !m.inspectHunkFocus
		if m.inspectHunkFocus {
			m.inspectTA.Blur()
			return m, nil
		}
		return m, m.inspectTA.Focus()
	}

	if m.inspectHunkFocus {
		var cmd tea.Cmd
		m.inspectHunkVP, cmd = m.inspectHunkVP.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.inspectTA, cmd = m.inspectTA.Update(msg)
	return m, cmd
}


func (m Model) updateOrphan(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		return m, nil

	case "r":
		return m.toggleOrphanResolved(true)

	case "u":
		return m.toggleOrphanResolved(false)

	case "o":
		if len(m.orphans) > 0 {
			m.orphanIdx = (m.orphanIdx + 1) % len(m.orphans)
			m.loadOrphanView()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.orphanVP, cmd = m.orphanVP.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	switch m.mode {
	case ModeHelp:
		v := tea.NewView(renderHelp(m.keymap, m.width, m.height))
		v.AltScreen = true
		return v
	case ModeComment:
		v := tea.NewView(renderCommentModal(m.commentTA, m.width, m.height))
		v.AltScreen = true
		return v
	case ModeInspect:
		c := m.findComment(m.inspectID)
		v := tea.NewView(renderInspectModal(c, m.inspectTA, m.inspectHunkVP, m.inspectHunkFocus, m.width, m.height))
		v.AltScreen = true
		return v
	case ModeDefinition:
		v := tea.NewView(viewDefinition(m))
		v.AltScreen = true
		return v
	default:
		v := tea.NewView(m.buildView())
		v.AltScreen = true
		return v
	}
}

func (m Model) buildView() string {
	var parts []string

	parts = append(parts, renderHeader(m.session, len(m.files)))

	if len(m.orphans) > 0 && m.mode != ModeOrphan {
		banner := orphanBannerStyle.Render(
			fmt.Sprintf("  ⚠  %d orphaned comment(s) — press o to review", len(m.orphans)))
		parts = append(parts, banner)
	}

	parts = append(parts, renderFileList(m.files, m.fileIdx, m.commentCountsByFile(), m.width))
	parts = append(parts, separatorStyle.Render(strings.Repeat("─", m.width)))

	vp := m.viewport
	fl := m.focusedLine
	w := m.width
	vp.StyleLineFunc = func(idx int) lipgloss.Style {
		if idx == fl {
			return selectionStyle.Width(w)
		}
		return lipgloss.NewStyle()
	}

	if l, ok := m.cursorTargetLine(); ok && len(m.renderedLines) > 0 {
		overlay := make([]string, len(m.renderedLines))
		copy(overlay, m.renderedLines)
		overlay[fl] = renderFocusedLineWithCursor(l, m.fileNumWidth, m.focusedCol)
		vp.SetContent(strings.Join(overlay, "\n"))
	}

	switch m.mode {
	case ModeOrphan:
		parts = append(parts, m.orphanVP.View())
	default:
		parts = append(parts, vp.View())
	}

	if m.statusMsg != "" {
		parts = append(parts, statusMsgStyle.Render("  "+m.statusMsg))
	} else {
		parts = append(parts, renderKeyBar(m.keymap, m.mode))
	}

	return strings.Join(parts, "\n")
}


// rebuildDiffView rebuilds the viewport content for the current file (or PR Comments view).
func (m *Model) rebuildDiffView() {
	if m.width == 0 {
		return
	}

	vpH := m.viewportHeight()
	m.viewport = viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(vpH))
	m.viewport.KeyMap.Up.SetEnabled(false)
	m.viewport.KeyMap.Down.SetEnabled(false)

	if m.fileIdx == -1 {
		result := renderPRComments(m.comments, m.width)
		m.viewport.SetContent(result.content)
		m.linesMeta = result.linesMeta
		m.hunkStarts = nil
		m.commentPositions = nil
		m.renderedLines = nil
		m.fileNumWidth = 0
		m.fileExt = ""
		m.focusedLine = 0
		m.focusedCol = 0
		m.lineIndex = nil
		return
	}

	if len(m.files) == 0 {
		m.lineIndex = nil
		return
	}

	file := m.files[m.fileIdx]
	result := renderDiff(file, m.comments, m.width)

	m.linesMeta = result.linesMeta
	m.hunkStarts = result.hunkStarts
	m.commentPositions = result.commentPositions
	m.renderedLines = result.lines
	m.fileNumWidth = result.numWidth
	m.fileExt = result.ext

	m.rebuildLineIndex(file)

	if len(m.linesMeta) > 0 && m.focusedLine >= len(m.linesMeta) {
		m.focusedLine = len(m.linesMeta) - 1
	}

	m.viewport.SetContent(result.content)
	m.ensureLineVisible()
}

// rebuildLineIndex populates m.lineIndex from the current file's linesMeta.
// Only Add/Context lines (with a NewLineNum > 0) are indexed — these are the
// lines that a go-to-definition jump can land on.
func (m *Model) rebuildLineIndex(file diff.ParsedFile) {
	m.lineIndex = map[string]map[int32]int{}
	byLine := map[int32]int{}
	for idx, meta := range m.linesMeta {
		if meta.isHunkHeader || meta.isComment {
			continue
		}
		if meta.hunkIdx < 0 || meta.hunkIdx >= len(file.Hunks) {
			continue
		}
		hunk := file.Hunks[meta.hunkIdx]
		if meta.lineInHunk < 0 || meta.lineInHunk >= len(hunk.Lines) {
			continue
		}
		l := hunk.Lines[meta.lineInHunk]
		if l.Kind == diff.LineDelete || l.NewLineNum <= 0 {
			continue
		}
		if _, exists := byLine[l.NewLineNum]; !exists {
			byLine[l.NewLineNum] = idx
		}
	}
	if len(byLine) > 0 {
		m.lineIndex[file.NewPath] = byLine
	}
}

// cursorTargetLine returns the parsed diff line under the cursor, plus ok=true,
// only if the focused line is a renderable diff line (not a hunk header or
// inline comment marker). The cursor is hidden and motion is a no-op otherwise.
func (m *Model) cursorTargetLine() (diff.ParsedLine, bool) {
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		return diff.ParsedLine{}, false
	}
	if m.focusedLine < 0 || m.focusedLine >= len(m.linesMeta) {
		return diff.ParsedLine{}, false
	}
	meta := m.linesMeta[m.focusedLine]
	if meta.isHunkHeader || meta.isComment {
		return diff.ParsedLine{}, false
	}
	if meta.hunkIdx < 0 || meta.hunkIdx >= len(m.files[m.fileIdx].Hunks) {
		return diff.ParsedLine{}, false
	}
	hunk := m.files[m.fileIdx].Hunks[meta.hunkIdx]
	if meta.lineInHunk < 0 || meta.lineInHunk >= len(hunk.Lines) {
		return diff.ParsedLine{}, false
	}
	return hunk.Lines[meta.lineInHunk], true
}

func (m *Model) moveCursor(delta int) {
	l, ok := m.cursorTargetLine()
	if !ok {
		return
	}
	maxCol := len([]rune(l.Text)) - 1
	if maxCol < 0 {
		m.focusedCol = 0
		return
	}
	m.focusedCol = clamp(m.focusedCol+delta, 0, maxCol)
}

func (m *Model) cursorWord(jump func(string, int) int) {
	l, ok := m.cursorTargetLine()
	if !ok {
		return
	}
	if len([]rune(l.Text)) == 0 {
		m.focusedCol = 0
		return
	}
	m.focusedCol = jump(l.Text, m.focusedCol)
}

func (m *Model) viewportHeight() int {
	if m.height == 0 {
		return 10
	}
	overhead := 2 // header + keybar
	overhead += fileListHeight(len(m.files))
	overhead++ // separator (always present alongside the file list)
	if len(m.orphans) > 0 && m.mode != ModeOrphan {
		overhead++ // orphan banner
	}
	h := m.height - overhead
	if h < 3 {
		h = 3
	}
	return h
}

func fileListHeight(fileCount int) int {
	// Always includes the virtual "PR Comments" entry (+1) and its separator (+1).
	total := fileCount + 1
	visible := total
	if visible > maxVisibleFiles {
		visible = maxVisibleFiles
	}
	if total >= 2 {
		return visible + 2 // entries + separator + nav hint
	}
	return visible + 1 // entry + separator
}

func (m *Model) moveLine(delta int) {
	if len(m.linesMeta) == 0 {
		return
	}
	m.focusedLine = clamp(m.focusedLine+delta, 0, len(m.linesMeta)-1)
	m.ensureLineVisible()
}

func (m *Model) jumpHunk(delta int) {
	if len(m.hunkStarts) == 0 {
		return
	}
	cur := m.focusedLine
	starts := m.hunkStarts

	if delta > 0 {
		for _, hs := range starts {
			if hs > cur {
				m.focusedLine = hs
				m.ensureLineVisible()
				return
			}
		}
		m.focusedLine = starts[0]
	} else {
		for i := len(starts) - 1; i >= 0; i-- {
			if starts[i] < cur {
				m.focusedLine = starts[i]
				m.ensureLineVisible()
				return
			}
		}
		m.focusedLine = starts[len(starts)-1]
	}
	m.ensureLineVisible()
}

func (m *Model) jumpComment(delta int) {
	if len(m.commentPositions) == 0 {
		return
	}
	positions := m.commentPositions
	cur := m.focusedLine

	if delta > 0 {
		for _, cp := range positions {
			if cp.renderedIdx > cur {
				m.focusedLine = cp.renderedIdx
				m.ensureLineVisible()
				return
			}
		}
		m.focusedLine = positions[0].renderedIdx
	} else {
		for i := len(positions) - 1; i >= 0; i-- {
			if positions[i].renderedIdx < cur {
				m.focusedLine = positions[i].renderedIdx
				m.ensureLineVisible()
				return
			}
		}
		m.focusedLine = positions[len(positions)-1].renderedIdx
	}
	m.ensureLineVisible()
}

func (m *Model) cycleFile(delta int) {
	if len(m.files) == 0 {
		return
	}
	// Virtual index -1 is the "PR Comments" entry before index 0.
	// Total entries = len(m.files) + 1 (the PR Comments entry).
	total := len(m.files) + 1
	// Shift to a 0-based range where 0 = PR Comments, 1..N = files 0..N-1.
	current := m.fileIdx + 1
	next := (current + delta + total) % total
	m.fileIdx = next - 1
	m.focusedLine = 0
	m.rebuildDiffView()
}

func (m *Model) ensureLineVisible() {
	const margin = 3
	fl := m.focusedLine
	vpTop := m.viewport.YOffset()
	vpBottom := vpTop + m.viewport.Height() - 1

	if fl < vpTop+margin {
		m.viewport.SetYOffset(max(0, fl-margin))
	} else if fl > vpBottom-margin {
		m.viewport.SetYOffset(fl - m.viewport.Height() + 1 + margin)
	}
}

func (m Model) enterCommentMode() (tea.Model, tea.Cmd) {
	if len(m.linesMeta) == 0 || len(m.files) == 0 {
		return m, nil
	}
	fl := m.focusedLine
	// Advance past hunk headers and inline comment lines to a real diff line.
	for fl < len(m.linesMeta) && (m.linesMeta[fl].isHunkHeader || m.linesMeta[fl].isComment) {
		fl++
	}
	if fl >= len(m.linesMeta) {
		m.statusMsg = "no commentable lines in this file"
		return m, nil
	}
	m.focusedLine = fl
	meta := m.linesMeta[fl]
	if meta.hunkIdx < 0 || meta.hunkIdx >= len(m.files[m.fileIdx].Hunks) {
		return m, nil
	}
	hunk := m.files[m.fileIdx].Hunks[meta.hunkIdx]
	if meta.lineInHunk < 0 || meta.lineInHunk >= len(hunk.Lines) {
		return m, nil
	}
	l := hunk.Lines[meta.lineInHunk]
	var side diff.Side
	if l.Kind == diff.LineDelete {
		side = diff.Left
	} else {
		side = diff.Right
	}

	ta := textarea.New()
	ta.Placeholder = "Enter comment... (Ctrl+S to submit, Esc to cancel)"
	taW := m.width - 8
	if taW > 80 {
		taW = 80
	}
	ta.SetWidth(taW)
	ta.SetHeight(5)

	m.commentTA = ta
	m.commentFileIdx = m.fileIdx
	m.commentHunkIdx = meta.hunkIdx
	m.commentSide = side
	m.mode = ModeComment

	return m, m.commentTA.Focus()
}

// enterPRCommentMode opens the comment textarea for a PR-level comment (no line anchor).
func (m Model) enterPRCommentMode() (tea.Model, tea.Cmd) {
	ta := textarea.New()
	ta.Placeholder = "Enter PR-level comment... (Ctrl+S to submit, Esc to cancel)"
	taW := m.width - 8
	if taW > 80 {
		taW = 80
	}
	ta.SetWidth(taW)
	ta.SetHeight(5)

	m.commentTA = ta
	m.commentFileIdx = -1
	m.commentHunkIdx = -1
	m.commentSide = diff.Right
	m.mode = ModeComment

	return m, m.commentTA.Focus()
}

func (m *Model) submitComment(body string) error {
	// PR-level comment: no file or line anchor.
	if m.commentFileIdx == -1 {
		c := diff.Comment{
			ID:          newUUID(),
			File:        "",
			HunkHeader:  "",
			LineHint:    0,
			Side:        "",
			Context:     diff.CommentContext{},
			Body:        body,
			Author:      resolveAuthor(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
			Resolved:    false,
			Orphaned:    false,
			SnapshotRef: currentSnapshotRef(m.session),
		}
		m.comments = append(m.comments, &c)
		if err := m.saveComments(); err != nil {
			m.comments = m.comments[:len(m.comments)-1]
			return err
		}
		m.rebuildDiffView()
		return nil
	}

	if m.commentFileIdx >= len(m.files) {
		return fmt.Errorf("invalid file index")
	}
	file := m.files[m.commentFileIdx]
	if m.commentHunkIdx >= len(file.Hunks) {
		return fmt.Errorf("invalid hunk index")
	}
	hunk := file.Hunks[m.commentHunkIdx]

	fl := m.focusedLine
	if fl >= len(m.linesMeta) {
		return fmt.Errorf("invalid focused line")
	}
	meta := m.linesMeta[fl]
	if meta.lineInHunk < 0 || meta.lineInHunk >= len(hunk.Lines) {
		return fmt.Errorf("invalid line in hunk")
	}
	anchor := diff.BuildAnchor(&hunk, meta.lineInHunk, m.commentSide)

	l := hunk.Lines[meta.lineInHunk]
	var lineHint int
	var side string
	if m.commentSide == diff.Left {
		lineHint = int(l.OldLineNum)
		side = "left"
	} else {
		lineHint = int(l.NewLineNum)
		side = "right"
	}

	c := diff.Comment{
		ID:          newUUID(),
		File:        file.NewPath,
		HunkHeader:  hunk.Header,
		LineHint:    lineHint,
		Side:        side,
		Context:     anchor,
		Body:        body,
		Author:      resolveAuthor(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Resolved:    false,
		Orphaned:    false,
		SnapshotRef: currentSnapshotRef(m.session),
	}

	m.comments = append(m.comments, &c)
	if err := m.saveComments(); err != nil {
		m.comments = m.comments[:len(m.comments)-1]
		return err
	}

	m.rebuildDiffView()
	return nil
}

func (m Model) enterOrphanMode() (tea.Model, tea.Cmd) {
	m.mode = ModeOrphan
	m.orphanIdx = 0
	m.loadOrphanView()
	return m, nil
}

func (m *Model) loadOrphanView() {
	if len(m.orphans) == 0 {
		return
	}
	orphanID := m.orphans[m.orphanIdx]
	var c *diff.Comment
	for _, comment := range m.comments {
		if comment.ID == orphanID {
			c = comment
			break
		}
	}
	if c == nil {
		return
	}
	content := m.renderOrphanContent(c)
	vpH := m.viewportHeight()
	m.orphanVP = viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(vpH))
	m.orphanVP.KeyMap.Up.SetEnabled(false)
	m.orphanVP.KeyMap.Down.SetEnabled(false)
	m.orphanVP.SetContent(content)
}

func (m *Model) renderOrphanContent(c *diff.Comment) string {
	var sb strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFAA00"))
	sb.WriteString(title.Render(fmt.Sprintf(
		"Orphaned Comment %d/%d", m.orphanIdx+1, len(m.orphans))) + "\n")
	sb.WriteString(keyHintDescStyle.Render(
		fmt.Sprintf("  File: %s  ·  Author: %s", c.File, c.Author)) + "\n\n")
	sb.WriteString(contextStyle.Render(c.Body) + "\n\n")
	sb.WriteString(title.Render("Original context:") + "\n")

	snapshotDiff := m.loadSnapshotDiff(c.SnapshotRef)
	if snapshotDiff != nil {
		for _, f := range snapshotDiff.Files {
			if f.NewPath != c.File && f.OldPath != c.File {
				continue
			}
			for _, h := range f.Hunks {
				if h.Header != c.HunkHeader {
					continue
				}
				sb.WriteString(hunkStyle.Render(h.Header) + "\n")
				for _, l := range h.Lines {
					sb.WriteString(renderOneDiffLine(l, 4, "") + "\n")
				}
				break
			}
			break
		}
	} else {
		sb.WriteString(hunkStyle.Render(c.HunkHeader) + "\n")
		for _, line := range c.Context.Before {
			sb.WriteString(keyHintDescStyle.Render("    "+line) + "\n")
		}
		sb.WriteString(commentMarkStyle.Render("  ▶ " + c.Context.Target) + "\n")
		for _, line := range c.Context.After {
			sb.WriteString(keyHintDescStyle.Render("    "+line) + "\n")
		}
	}
	return sb.String()
}

func (m *Model) loadSnapshotDiff(snapshotRef string) *diff.ParsedDiff {
	if snapshotRef == "" || m.session == nil {
		return nil
	}
	idx := strings.Index(snapshotRef, "_")
	if idx < 0 {
		return nil
	}
	baseSHA := snapshotRef[:idx]
	headSHA := snapshotRef[idx+1:]
	snapDir := diff.SnapshotDir(m.org, m.repoName, m.session.PRNumber, baseSHA, headSHA)
	data, err := os.ReadFile(snapDir + "/diff.patch")
	if err != nil {
		return nil
	}
	pd, err := diff.Parse(data)
	if err != nil {
		return nil
	}
	return pd
}

func (m Model) toggleOrphanResolved(resolved bool) (tea.Model, tea.Cmd) {
	if len(m.orphans) == 0 {
		return m, nil
	}
	orphanID := m.orphans[m.orphanIdx]
	for _, c := range m.comments {
		if c.ID == orphanID {
			c.Resolved = resolved
			c.UpdatedAt = time.Now().UTC()
			break
		}
	}
	if err := m.saveComments(); err != nil {
		m.statusMsg = fmt.Sprintf("error saving: %v", err)
		return m, nil
	}
	if resolved {
		m.statusMsg = "comment resolved"
	} else {
		m.statusMsg = "comment unresolved"
	}
	return m, clearStatusCmd()
}

func (m *Model) saveComments() error {
	flat := make([]diff.Comment, len(m.comments))
	for i, c := range m.comments {
		flat[i] = *c
	}
	return diff.SaveComments(m.org, m.repoName, m.session.PRNumber, flat)
}

func (m *Model) toggleCommentResolved(id string, resolved bool) tea.Cmd {
	for _, c := range m.comments {
		if c.ID == id {
			c.Resolved = resolved
			_ = m.saveComments()
			m.rebuildDiffView()
			if resolved {
				m.statusMsg = "comment resolved"
			} else {
				m.statusMsg = "comment unresolved"
			}
			return clearStatusCmd()
		}
	}
	return nil
}

// hunkLines finds the hunk for a comment and returns its rendered lines.
func (m Model) hunkLines(c *diff.Comment) []string {
	if c == nil {
		return nil
	}
	for _, f := range m.files {
		if f.NewPath != c.File && f.OldPath != c.File {
			continue
		}
		for _, h := range f.Hunks {
			if h.Header != c.HunkHeader {
				continue
			}
			ext := filepath.Ext(f.NewPath)
			maxLine := 0
			for _, l := range h.Lines {
				if int(l.NewLineNum) > maxLine {
					maxLine = int(l.NewLineNum)
				}
				if int(l.OldLineNum) > maxLine {
					maxLine = int(l.OldLineNum)
				}
			}
			numWidth := len(fmt.Sprintf("%d", maxLine))
			if numWidth < 3 {
				numWidth = 3
			}
			lines := make([]string, len(h.Lines))
			for i, l := range h.Lines {
				lines[i] = renderOneDiffLine(l, numWidth, ext)
			}
			return lines
		}
	}
	return nil
}

func (m Model) commentCountsByFile() map[string]int {
	counts := make(map[string]int)
	for _, c := range m.comments {
		if !c.Orphaned {
			counts[c.File]++
		}
	}
	return counts
}

func (m Model) findComment(id string) *diff.Comment {
	for _, c := range m.comments {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// Exported accessors for testing.

func (m Model) FileIdx() int      { return m.fileIdx }
func (m Model) FocusedLine() int  { return m.focusedLine }
func (m Model) FocusedCol() int   { return m.focusedCol }
func (m Model) CurrentMode() Mode { return m.mode }
func (m Model) StatusMsg() string { return m.statusMsg }

// splitRepoLocal splits "org/repo" into components.
func splitRepoLocal(repo string) (org, name string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", repo
}

// currentSnapshotRef returns the base_head ref for the latest snapshot.
func currentSnapshotRef(session *diff.Session) string {
	if len(session.Snapshots) == 0 {
		return ""
	}
	snap := session.Snapshots[len(session.Snapshots)-1]
	return snap.Base + "_" + snap.Head
}

// resolveAuthor gets the author from git config or $USER.
func resolveAuthor() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "unknown"
}

// newUUID generates a random UUID v4.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ tea.Model = Model{}
