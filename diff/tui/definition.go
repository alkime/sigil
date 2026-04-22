package tui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alkime/sigil/diff"
	"github.com/alkime/sigil/lsp"
)

const jumpHistoryCap = 32

// Status-message strings for LSP / go-to-definition flows. Centralised here so
// copy changes only touch one place.
const (
	msgNoSymbol          = "no symbol under cursor"
	msgNoLSPConfigured   = "no LSP configured for %s"
	msgInitializingGopls = "initializing gopls..." // reserved: emitted when we surface server init progress
	msgResolvingDef      = "resolving definition..."
	msgNoDefinition      = "no definition found"
	msgDeletedLine       = "go-to-def on deleted lines not supported"
	msgLSPDisabled       = "LSP disabled: worktree not found"
)

// defResultMsg is emitted asynchronously by goToDefinition's tea.Cmd when the
// LSP roundtrip completes (or fails).
type defResultMsg struct {
	sym       string
	locations []lsp.Location
	err       error
}

// jumpEntry records a location pushed onto the jump stack for ctrl+o / q
// restore. defFile/defLine are populated when the entry represents a previous
// ModeDefinition frame; otherwise they are zero.
type jumpEntry struct {
	mode        Mode
	fileIdx     int
	focusedLine int
	focusedCol  int
	defFile     string
	defLine     int
}

// lineKindToSide returns the diff side that a given line kind belongs to.
// Delete lines belong to the left (old) side; context and add lines to the right (new) side.
func lineKindToSide(k diff.LineKind) diff.Side {
	if k == diff.LineDelete {
		return diff.Left
	}
	return diff.Right
}

// goToDefinition inspects the cursor position, extracts the symbol under it,
// and returns a tea.Cmd that performs the async LSP definition request. On
// any synchronous failure it sets m.statusMsg and returns nil.
func (m *Model) goToDefinition() tea.Cmd {
	if m.focusedLine < 0 || m.focusedLine >= len(m.linesMeta) {
		m.statusMsg = msgNoSymbol
		return nil
	}
	meta := m.linesMeta[m.focusedLine]
	if meta.isHunkHeader || meta.isComment {
		m.statusMsg = msgNoSymbol
		return nil
	}
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		m.statusMsg = msgNoSymbol
		return nil
	}
	file := m.files[m.fileIdx]
	if meta.hunkIdx < 0 || meta.hunkIdx >= len(file.Hunks) {
		m.statusMsg = msgNoSymbol
		return nil
	}
	hunk := file.Hunks[meta.hunkIdx]
	if meta.lineInHunk < 0 || meta.lineInHunk >= len(hunk.Lines) {
		m.statusMsg = msgNoSymbol
		return nil
	}
	line := hunk.Lines[meta.lineInHunk]

	start, end, ok := WordAt(line.Text, m.focusedCol)
	if !ok {
		m.statusMsg = msgNoSymbol
		return nil
	}
	runes := []rune(line.Text)
	sym := string(runes[start : end+1])

	if lineKindToSide(meta.lineKind) == diff.Left {
		m.statusMsg = msgDeletedLine
		return nil
	}

	if m.worktreePath == "" {
		m.statusMsg = msgLSPDisabled
		return nil
	}

	absFile := filepath.Join(m.worktreePath, file.NewPath)
	lspLine := int(line.NewLineNum - 1)
	lspCol := m.focusedCol
	return m.lspDefinitionRequest(absFile, lspLine, lspCol, sym)
}

// lspDefinitionRequest builds and returns the tea.Cmd that performs the async
// LSP textDocument/definition request. Shared between the diff-mode entry
// point and the ModeDefinition recursive entry point.
func (m *Model) lspDefinitionRequest(absFile string, lspLine, lspCol int, sym string) tea.Cmd {
	ext := filepath.Ext(absFile)
	cfg, ok := lsp.ForExtension(ext)
	if !ok {
		if ext == "" {
			ext = "(no extension)"
		}
		m.statusMsg = fmt.Sprintf(msgNoLSPConfigured, ext)
		return nil
	}

	root := resolveProjectRoot(absFile, cfg.RootMarkers, m.worktreePath)

	if m.lspManager == nil {
		m.lspManager = lsp.NewManager()
	}

	if m.lspReqCancel != nil {
		m.lspReqCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.lspReqCancel = cancel

	manager := m.lspManager
	m.statusMsg = msgResolvingDef

	return func() tea.Msg {
		client, err := manager.Get(ctx, cfg, root)
		if err != nil {
			return defResultMsg{sym: sym, err: err}
		}
		locs, err := client.Definition(ctx, absFile, lspLine, lspCol)
		return defResultMsg{sym: sym, locations: locs, err: err}
	}
}

// resolveProjectRoot walks up from absFile's directory looking for any of the
// given marker files; returns the first ancestor that contains one, or fallback.
func resolveProjectRoot(absFile string, markers []string, fallback string) string {
	dir := filepath.Dir(absFile)
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return fallback
}

// handleDefResult routes a completed definition response to the right place:
// error/empty → status bar; single → dispatchLocation; multiple → pick first
// (with status bar note; a full multi-picker is future work).
func (m *Model) handleDefResult(msg defResultMsg) tea.Cmd {
	if msg.err != nil {
		// A superseded request returns context.Canceled; swallow it so a
		// stale response doesn't clobber the new request's status.
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		m.statusMsg = msg.err.Error()
		return clearStatusCmd()
	}
	if len(msg.locations) == 0 {
		m.statusMsg = msgNoDefinition
		return clearStatusCmd()
	}
	// Multi-location picker is future work. For now, pick the first location
	// and note when more exist.
	if len(msg.locations) > 1 {
		m.statusMsg = fmt.Sprintf("%d definitions found; jumping to first", len(msg.locations))
	}
	return m.dispatchLocation(msg.locations[0], msg.sym)
}

// dispatchLocation routes a resolved location either to an in-diff scroll or
// to the out-of-diff ModeDefinition viewer. Returns a tea.Cmd to clear the
// status line after a successful in-diff jump.
func (m *Model) dispatchLocation(loc lsp.Location, sym string) tea.Cmd {
	absPath := uriToPath(loc.URI)
	if absPath == "" {
		m.statusMsg = "invalid definition URI"
		return clearStatusCmd()
	}
	newLineNum := int32(loc.Range.Start.Line + 1)

	var relPath string
	if m.worktreePath != "" {
		if rel, err := filepath.Rel(m.worktreePath, absPath); err == nil {
			relPath = filepath.ToSlash(rel)
		}
	}

	if relPath != "" && !strings.HasPrefix(relPath, "..") {
		if m.jumpToInDiff(relPath, newLineNum) {
			return clearStatusCmd()
		}
	}

	openDefinition(m, loc, sym)
	return nil
}

// jumpToInDiff attempts to locate (relPath, newLineNum) in the current diff
// and focus it. Returns true on success; false when no in-diff match exists.
func (m *Model) jumpToInDiff(relPath string, newLineNum int32) bool {
	targetFileIdx := -2 // -1 is reserved for PR Comments, so use -2 for "not found".
	for i, f := range m.files {
		if f.NewPath == relPath {
			targetFileIdx = i
			break
		}
	}
	if targetFileIdx == -2 {
		return false
	}

	// If the target file is already rendered, look up the line in the current index.
	if targetFileIdx == m.fileIdx {
		if byLine, ok := m.lineIndex[relPath]; ok {
			if idx, ok := byLine[newLineNum]; ok {
				m.pushJump(m.mode, m.fileIdx, m.focusedLine, m.focusedCol)
				m.focusedLine = idx
				m.focusedCol = 0
				m.ensureLineVisible()
				m.statusMsg = "jumped to definition"
				return true
			}
		}
		return false
	}

	// Cross-file jump: rebuild into the target file, then look up.
	m.pushJump(m.mode, m.fileIdx, m.focusedLine, m.focusedCol)
	m.fileIdx = targetFileIdx
	m.rebuildDiffView()
	if byLine, ok := m.lineIndex[relPath]; ok {
		if idx, ok := byLine[newLineNum]; ok {
			m.focusedLine = idx
			m.focusedCol = 0
			m.ensureLineVisible()
			m.statusMsg = "jumped to definition"
			return true
		}
	}
	// Landed on the file but the target line isn't in any hunk; caller will
	// treat this as a failed in-diff match. Pop the jump we speculatively pushed.
	if n := len(m.jumpHistory); n > 0 {
		m.jumpHistory = m.jumpHistory[:n-1]
	}
	return false
}

// uriToPath converts a file:// URI to an absolute filesystem path, handling
// percent-encoding. Returns "" on parse failure.
func uriToPath(uri string) string {
	if uri == "" {
		return ""
	}
	if strings.HasPrefix(uri, "file://") {
		u, err := url.Parse(uri)
		if err != nil {
			return ""
		}
		return u.Path
	}
	return uri
}

// insideWorktree reports whether absPath resides within worktree. Empty
// worktree → always false (the feature degrades to [external]).
func insideWorktree(absPath, worktree string) bool {
	if worktree == "" {
		return false
	}
	if absPath == worktree {
		return true
	}
	return strings.HasPrefix(absPath, worktree+string(filepath.Separator))
}

// pushJumpEntry records a jumpEntry on the bounded jump-history stack.
func (m *Model) pushJumpEntry(e jumpEntry) {
	m.jumpHistory = append(m.jumpHistory, e)
	if len(m.jumpHistory) > jumpHistoryCap {
		m.jumpHistory = m.jumpHistory[len(m.jumpHistory)-jumpHistoryCap:]
	}
}

// pushJump is a convenience wrapper for diff-mode callers that don't track a
// ModeDefinition frame.
func (m *Model) pushJump(mode Mode, fileIdx, focusedLine, focusedCol int) {
	m.pushJumpEntry(jumpEntry{
		mode:        mode,
		fileIdx:     fileIdx,
		focusedLine: focusedLine,
		focusedCol:  focusedCol,
	})
}

// popJump restores the top jump-history entry; returns false when empty. When
// the top entry represents a prior ModeDefinition frame, the viewer is
// reopened at that file/line; otherwise the diff view is restored.
func (m *Model) popJump() bool {
	if len(m.jumpHistory) == 0 {
		return false
	}
	entry := m.jumpHistory[len(m.jumpHistory)-1]
	m.jumpHistory = m.jumpHistory[:len(m.jumpHistory)-1]

	if entry.mode == ModeDefinition {
		if err := openDefAt(m, entry.defFile, entry.defLine, m.defSymbol); err != nil {
			m.statusMsg = fmt.Sprintf("cannot reopen %s: %v", entry.defFile, err)
			clearDefState(m)
			m.mode = ModeNormal
			return true
		}
		m.focusedCol = entry.focusedCol
		return true
	}

	clearDefState(m)
	m.mode = entry.mode
	if entry.fileIdx != m.fileIdx {
		m.fileIdx = entry.fileIdx
		m.rebuildDiffView()
	}
	if entry.focusedLine >= 0 && entry.focusedLine < len(m.linesMeta) {
		m.focusedLine = entry.focusedLine
	}
	m.focusedCol = entry.focusedCol
	m.ensureLineVisible()
	return true
}

// defViewportHeight sizes the ModeDefinition viewport: total height minus
// header + separator + status bar.
func defViewportHeight(m *Model) int {
	if m.height == 0 {
		return 10
	}
	h := m.height - 3
	if h < 3 {
		h = 3
	}
	return h
}

// openDefAt reads absPath, builds a syntax-highlighted viewport centred on
// line (1-indexed), and installs it as the active ModeDefinition state. It
// does not touch jumpHistory — callers manage the stack.
func openDefAt(m *Model, absPath string, line int, symbol string) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	ext := filepath.Ext(absPath)

	rendered := make([]string, len(raw))
	for i, text := range raw {
		rendered[i] = highlightLine(text, ext)
	}

	vpH := defViewportHeight(m)
	width := m.width
	if width <= 0 {
		width = 80
	}
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(vpH))
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.Down.SetEnabled(false)
	vp.SetContent(strings.Join(rendered, "\n"))

	targetIdx := line - 1
	if targetIdx < 0 {
		targetIdx = 0
	}
	if len(raw) > 0 && targetIdx >= len(raw) {
		targetIdx = len(raw) - 1
	}
	offset := targetIdx - vpH/2
	maxOffset := len(raw) - vpH
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	vp.SetYOffset(offset)

	m.defFile = absPath
	m.defLine = targetIdx + 1
	m.defSymbol = symbol
	m.defLines = raw
	m.defMeta = nil
	m.defVP = vp
	m.defIsExternal = !insideWorktree(absPath, m.worktreePath)
	m.focusedCol = 0
	m.mode = ModeDefinition
	return nil
}

// openDefinition switches the TUI into ModeDefinition, showing the file at
// loc.URI scrolled to and highlighting loc.Range.Start.Line. It pushes a
// jumpEntry for the caller's current location so q/Esc can restore it.
// Failure to read the target file sets m.statusMsg and rolls back the jump.
func openDefinition(m *Model, loc lsp.Location, symbol string) {
	absPath := uriToPath(loc.URI)
	if absPath == "" {
		m.statusMsg = "invalid definition URI"
		return
	}

	m.pushJumpEntry(jumpEntry{
		mode:        m.mode,
		fileIdx:     m.fileIdx,
		focusedLine: m.focusedLine,
		focusedCol:  m.focusedCol,
		defFile:     m.defFile,
		defLine:     m.defLine,
	})

	if err := openDefAt(m, absPath, loc.Range.Start.Line+1, symbol); err != nil {
		// Roll back the speculative jump.
		if n := len(m.jumpHistory); n > 0 {
			m.jumpHistory = m.jumpHistory[:n-1]
		}
		m.statusMsg = fmt.Sprintf("cannot read %s: %v", absPath, err)
		return
	}
	m.statusMsg = ""
}

// clearDefState resets ModeDefinition fields to their zero values. Callers
// should set m.mode afterwards.
func clearDefState(m *Model) {
	m.defFile = ""
	m.defLine = 0
	m.defSymbol = ""
	m.defLines = nil
	m.defMeta = nil
	m.defIsExternal = false
	m.defVP = viewport.Model{}
}

// updateDefinition handles keys while ModeDefinition is active.
func updateDefinition(m *Model, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keyStr := msg.String()

	// 'gd' chord fires recursive go-to-definition on the cursor symbol.
	if keyStr == "g" {
		m.lastKey = "g"
		return *m, nil
	}
	if keyStr == "d" && m.lastKey == "g" {
		m.lastKey = ""
		cmd := recurseDefinition(m)
		return *m, cmd
	}
	m.lastKey = ""

	switch keyStr {
	case "q", "esc":
		if !m.popJump() {
			clearDefState(m)
			m.mode = ModeNormal
		}
		return *m, nil

	case "ctrl+o":
		if !m.popJump() {
			m.statusMsg = "no previous location"
		}
		return *m, nil

	case "ctrl+]":
		cmd := recurseDefinition(m)
		return *m, cmd

	case "j", "down":
		m.defVP.SetYOffset(m.defVP.YOffset() + 1)
		return *m, nil
	case "k", "up":
		off := m.defVP.YOffset() - 1
		if off < 0 {
			off = 0
		}
		m.defVP.SetYOffset(off)
		return *m, nil
	case "J":
		m.defVP.SetYOffset(m.defVP.YOffset() + m.defVP.Height()/2)
		return *m, nil
	case "K":
		off := m.defVP.YOffset() - m.defVP.Height()/2
		if off < 0 {
			off = 0
		}
		m.defVP.SetYOffset(off)
		return *m, nil

	case "h":
		moveDefCursor(m, -1)
		return *m, nil
	case "l":
		moveDefCursor(m, 1)
		return *m, nil
	case "w":
		defCursorWord(m, WordNext)
		return *m, nil
	case "b":
		defCursorWord(m, WordPrev)
		return *m, nil
	case "e":
		defCursorWord(m, WordEnd)
		return *m, nil
	}

	var cmd tea.Cmd
	m.defVP, cmd = m.defVP.Update(msg)
	return *m, cmd
}

// defTargetText returns the raw text of the currently-focused definition line.
func defTargetText(m *Model) (string, bool) {
	if m.defLine < 1 || m.defLine > len(m.defLines) {
		return "", false
	}
	return m.defLines[m.defLine-1], true
}

func moveDefCursor(m *Model, delta int) {
	text, ok := defTargetText(m)
	if !ok {
		return
	}
	runes := []rune(text)
	maxCol := len(runes) - 1
	if maxCol < 0 {
		m.focusedCol = 0
		return
	}
	m.focusedCol = clamp(m.focusedCol+delta, 0, maxCol)
}

func defCursorWord(m *Model, jump func(string, int) int) {
	text, ok := defTargetText(m)
	if !ok {
		return
	}
	if len([]rune(text)) == 0 {
		m.focusedCol = 0
		return
	}
	m.focusedCol = jump(text, m.focusedCol)
}

// recurseDefinition triggers LSP go-to-definition on the symbol at
// m.focusedCol inside the ModeDefinition viewer.
func recurseDefinition(m *Model) tea.Cmd {
	text, ok := defTargetText(m)
	if !ok {
		m.statusMsg = msgNoSymbol
		return nil
	}
	start, end, ok := WordAt(text, m.focusedCol)
	if !ok {
		m.statusMsg = msgNoSymbol
		return nil
	}
	runes := []rune(text)
	sym := string(runes[start : end+1])

	absFile := m.defFile
	lspLine := m.defLine - 1
	lspCol := m.focusedCol
	return m.lspDefinitionRequest(absFile, lspLine, lspCol, sym)
}

// viewDefinition renders ModeDefinition: header + separator + viewport body
// + status bar / key hints.
func viewDefinition(m Model) string {
	label := fmt.Sprintf("%s:%d  definition of %s", m.defFile, m.defLine, m.defSymbol)
	if m.defIsExternal {
		label = "[external] " + label
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		Render(label)

	vp := m.defVP
	targetIdx := m.defLine - 1
	w := m.width
	vp.StyleLineFunc = func(idx int) lipgloss.Style {
		if idx == targetIdx {
			return selectionStyle.Width(w)
		}
		return lipgloss.NewStyle()
	}

	sep := separatorStyle.Render(strings.Repeat("─", m.width))
	parts := []string{title, sep, vp.View()}
	if m.statusMsg != "" {
		parts = append(parts, statusMsgStyle.Render("  "+m.statusMsg))
	} else {
		parts = append(parts, renderKeyBar(m.keymap, m.mode, m.width))
	}
	return strings.Join(parts, "\n")
}
