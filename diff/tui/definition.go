package tui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alkime/sigil/diff"
	"github.com/alkime/sigil/lsp"
)

const jumpHistoryCap = 32

// defResultMsg is emitted asynchronously by goToDefinition's tea.Cmd when the
// LSP roundtrip completes (or fails).
type defResultMsg struct {
	sym       string
	locations []lsp.Location
	err       error
}

// jumpEntry records a location pushed onto the jump stack for ctrl+o restore.
type jumpEntry struct {
	mode        Mode
	fileIdx     int
	focusedLine int
	focusedCol  int
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
// any synchronous failure (no symbol, unsupported side, no LSP config, missing
// worktree) it sets m.statusMsg and returns nil.
func (m *Model) goToDefinition() tea.Cmd {
	if m.focusedLine < 0 || m.focusedLine >= len(m.linesMeta) {
		m.statusMsg = "no symbol under cursor"
		return nil
	}
	meta := m.linesMeta[m.focusedLine]
	if meta.isHunkHeader || meta.isComment {
		m.statusMsg = "no symbol under cursor"
		return nil
	}
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		m.statusMsg = "no symbol under cursor"
		return nil
	}
	file := m.files[m.fileIdx]
	if meta.hunkIdx < 0 || meta.hunkIdx >= len(file.Hunks) {
		m.statusMsg = "no symbol under cursor"
		return nil
	}
	hunk := file.Hunks[meta.hunkIdx]
	if meta.lineInHunk < 0 || meta.lineInHunk >= len(hunk.Lines) {
		m.statusMsg = "no symbol under cursor"
		return nil
	}
	line := hunk.Lines[meta.lineInHunk]

	start, end, ok := WordAt(line.Text, m.focusedCol)
	if !ok {
		m.statusMsg = "no symbol under cursor"
		return nil
	}
	runes := []rune(line.Text)
	sym := string(runes[start : end+1])

	if lineKindToSide(meta.lineKind) == diff.Left {
		m.statusMsg = "go-to-def on deleted lines not supported"
		return nil
	}

	if m.worktreePath == "" {
		m.statusMsg = "LSP disabled: worktree not found"
		return nil
	}

	absFile := filepath.Join(m.worktreePath, file.NewPath)
	ext := filepath.Ext(absFile)
	cfg, ok := lsp.ForExtension(ext)
	if !ok {
		if ext == "" {
			ext = "(no extension)"
		}
		m.statusMsg = fmt.Sprintf("no LSP configured for %s", ext)
		return nil
	}

	lspLine := int(line.NewLineNum - 1)
	lspCol := m.focusedCol

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
	m.statusMsg = "resolving definition..."

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
		m.statusMsg = "no definition found"
		return clearStatusCmd()
	}
	// Multi-location picker is future work (sig-div5 / polish). For now,
	// pick the first location and note when more exist.
	if len(msg.locations) > 1 {
		m.statusMsg = fmt.Sprintf("%d definitions found; jumping to first", len(msg.locations))
	}
	m.dispatchLocation(msg.locations[0])
	return nil
}

// dispatchLocation routes a resolved location either to an in-diff scroll or
// to an out-of-diff stub (to be wired to ModeDefinition by stream 5).
func (m *Model) dispatchLocation(loc lsp.Location) {
	absPath := uriToPath(loc.URI)
	if absPath == "" {
		m.statusMsg = "invalid definition URI"
		return
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
			return
		}
	}

	// TODO(sig-0dj6): route to ModeDefinition / OpenDefinition once stream 5 lands.
	m.pushJump(m.mode, m.fileIdx, m.focusedLine, m.focusedCol)
	m.statusMsg = fmt.Sprintf("out-of-diff target: %s:%d", absPath, newLineNum)
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

// pushJump records a location on the bounded jump-history stack.
func (m *Model) pushJump(mode Mode, fileIdx, focusedLine, focusedCol int) {
	m.jumpHistory = append(m.jumpHistory, jumpEntry{
		mode:        mode,
		fileIdx:     fileIdx,
		focusedLine: focusedLine,
		focusedCol:  focusedCol,
	})
	if len(m.jumpHistory) > jumpHistoryCap {
		m.jumpHistory = m.jumpHistory[len(m.jumpHistory)-jumpHistoryCap:]
	}
}

// popJump restores the top jump-history entry; returns false when empty.
func (m *Model) popJump() bool {
	if len(m.jumpHistory) == 0 {
		return false
	}
	entry := m.jumpHistory[len(m.jumpHistory)-1]
	m.jumpHistory = m.jumpHistory[:len(m.jumpHistory)-1]

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
