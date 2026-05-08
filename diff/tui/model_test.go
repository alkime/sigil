package tui_test

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/alkime/sigil/diff"
	dtui "github.com/alkime/sigil/diff/tui"
)

// --- helpers ---

func makeSession() *diff.Session {
	return &diff.Session{
		ID:         "test-session-id",
		Repo:       "testorg/testrepo",
		PRNumber:   42,
		PRTitle:    "Test PR",
		BaseBranch: "main",
		BaseSHA:    "aaa",
		HeadSHA:    "bbb",
		Branch:     "feature/test",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

func makeParsedDiff() *diff.ParsedDiff {
	return &diff.ParsedDiff{
		Files: []diff.ParsedFile{
			{
				OldPath: "a/foo.go",
				NewPath: "foo.go",
				Hunks: []diff.ParsedHunk{
					{
						OldStart: 1, OldLines: 3, NewStart: 1, NewLines: 4,
						Header: "@@ -1,3 +1,4 @@",
						Lines: []diff.ParsedLine{
							{Kind: diff.LineContext, OldLineNum: 1, NewLineNum: 1, Text: "package main"},
							{Kind: diff.LineAdd, OldLineNum: 0, NewLineNum: 2, Text: `import "fmt"`},
							{Kind: diff.LineContext, OldLineNum: 2, NewLineNum: 3, Text: ""},
							{Kind: diff.LineContext, OldLineNum: 3, NewLineNum: 4, Text: "func main() {}"},
						},
					},
					{
						OldStart: 10, OldLines: 2, NewStart: 11, NewLines: 2,
						Header: "@@ -10,2 +11,2 @@",
						Lines: []diff.ParsedLine{
							{Kind: diff.LineDelete, OldLineNum: 10, NewLineNum: 0, Text: "// old comment"},
							{Kind: diff.LineAdd, OldLineNum: 0, NewLineNum: 11, Text: "// new comment"},
						},
					},
				},
			},
			{
				OldPath: "a/bar.go",
				NewPath: "bar.go",
				Hunks: []diff.ParsedHunk{
					{
						OldStart: 5, OldLines: 1, NewStart: 5, NewLines: 1,
						Header: "@@ -5,1 +5,1 @@",
						Lines: []diff.ParsedLine{
							{Kind: diff.LineAdd, OldLineNum: 0, NewLineNum: 5, Text: "// bar"},
						},
					},
				},
			},
		},
	}
}

// pressKey returns a tea.KeyPressMsg from a key string.
func pressKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	default:
		if len(s) == 1 {
			r := rune(s[0])
			return tea.KeyPressMsg{Code: r, Text: s}
		}
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
}

// send applies a sequence of key presses to the model and returns the final model.
func send(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		m, _ = m.Update(pressKey(k))
	}
	return m
}

// setupModel creates an initialized diff TUI model with size applied.
func setupModel(t *testing.T) dtui.Model {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := makeSession()
	pd := makeParsedDiff()
	m := dtui.New(session, pd, "")
	out, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return out.(dtui.Model)
}

func viewText(m tea.Model) string {
	return ansi.Strip(m.(dtui.Model).View().Content)
}

// --- tests ---

func TestNew_initialState(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := makeSession()
	pd := makeParsedDiff()
	m := dtui.New(session, pd, "")

	if m.FileIdx() != 0 {
		t.Errorf("fileIdx = %d, want 0", m.FileIdx())
	}
	if m.FocusedLine() != 0 {
		t.Errorf("focusedLine = %d, want 0", m.FocusedLine())
	}
	if m.CurrentMode() != dtui.ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", m.CurrentMode())
	}
}

func TestNavigation_jk(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// j moves down
	m2 := send(m, "j").(dtui.Model)
	if m2.FocusedLine() <= m.FocusedLine() {
		t.Errorf("j did not move focusedLine down: before=%d after=%d", m.FocusedLine(), m2.FocusedLine())
	}

	// k moves back up
	m3 := send(m2, "k").(dtui.Model)
	if m3.FocusedLine() >= m2.FocusedLine() {
		t.Errorf("k did not move focusedLine up: before=%d after=%d", m2.FocusedLine(), m3.FocusedLine())
	}
}

func TestNavigation_jk_clampAtBounds(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// k at top should stay at 0
	m2 := send(m, "k").(dtui.Model)
	if m2.FocusedLine() != 0 {
		t.Errorf("k at top: focusedLine = %d, want 0", m2.FocusedLine())
	}

	// j many times should clamp at last line
	var cur tea.Model = m
	for i := 0; i < 100; i++ {
		cur, _ = cur.Update(pressKey("j"))
	}
	cur2 := send(cur, "j").(dtui.Model)
	if cur2.FocusedLine() != cur.(dtui.Model).FocusedLine() {
		t.Errorf("j past end: focusedLine changed past last line")
	}
}

func TestNavigation_Tab_cyclesFiles(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	if m.FileIdx() != 0 {
		t.Fatalf("initial fileIdx = %d, want 0", m.FileIdx())
	}

	// Tab from file 0 → file 1.
	m2 := send(m, "tab").(dtui.Model)
	if m2.FileIdx() != 1 {
		t.Errorf("after Tab: fileIdx = %d, want 1", m2.FileIdx())
	}

	// Tab from file 1 → PR Comments (virtual index -1).
	m3 := send(m2, "tab").(dtui.Model)
	if m3.FileIdx() != -1 {
		t.Errorf("after second Tab: fileIdx = %d, want -1 (PR Comments)", m3.FileIdx())
	}

	// Tab from PR Comments → file 0 (wrapped).
	m4 := send(m3, "tab").(dtui.Model)
	if m4.FileIdx() != 0 {
		t.Errorf("after third Tab: fileIdx = %d, want 0 (wrapped back to first file)", m4.FileIdx())
	}
}

func TestNavigation_JK_hunks(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// J should jump to the second hunk
	m2 := send(m, "J").(dtui.Model)
	// Should be past the first hunk
	if m2.FocusedLine() <= m.FocusedLine() {
		t.Errorf("J did not advance to next hunk: before=%d after=%d", m.FocusedLine(), m2.FocusedLine())
	}

	// K should go back
	m3 := send(m2, "K").(dtui.Model)
	if m3.FocusedLine() >= m2.FocusedLine() {
		t.Errorf("K did not go to prev hunk: before=%d after=%d", m2.FocusedLine(), m3.FocusedLine())
	}
}

func TestCursor_hlMovesAndClamps(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Move past hunk header to a renderable diff line.
	m1 := send(m, "j").(dtui.Model)
	if m1.FocusedCol() != 0 {
		t.Fatalf("initial focusedCol after j = %d, want 0", m1.FocusedCol())
	}

	// l moves cursor right.
	m2 := send(m1, "l").(dtui.Model)
	if m2.FocusedCol() != 1 {
		t.Errorf("after l: focusedCol = %d, want 1", m2.FocusedCol())
	}

	// h at col 0 stays at 0 (clamp).
	m3 := send(m1, "h").(dtui.Model)
	if m3.FocusedCol() != 0 {
		t.Errorf("h at col 0: focusedCol = %d, want 0 (clamped)", m3.FocusedCol())
	}

	// Spamming l clamps at len(text)-1.
	cur := tea.Model(m1)
	for i := 0; i < 200; i++ {
		cur, _ = cur.Update(pressKey("l"))
	}
	if cur.(dtui.Model).FocusedCol() < 1 {
		t.Errorf("after spamming l: focusedCol = %d, want >= 1", cur.(dtui.Model).FocusedCol())
	}

	// Line motion (j) resets focusedCol.
	m4 := send(cur, "j").(dtui.Model)
	if m4.FocusedCol() != 0 {
		t.Errorf("after j: focusedCol = %d, want 0 (reset)", m4.FocusedCol())
	}
}

func TestCursor_wbeWordMotions(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Land on first hunk line "package main" (focusedLine 1, col 0).
	m1 := send(m, "j").(dtui.Model)
	if m1.FocusedCol() != 0 {
		t.Fatalf("setup focusedCol = %d, want 0", m1.FocusedCol())
	}

	// w jumps to start of next word ("main" at col 8).
	m2 := send(m1, "w").(dtui.Model)
	if m2.FocusedCol() != 8 {
		t.Errorf("after w: focusedCol = %d, want 8", m2.FocusedCol())
	}

	// b jumps back to start of previous word ("package" at col 0).
	m3 := send(m2, "b").(dtui.Model)
	if m3.FocusedCol() != 0 {
		t.Errorf("after b: focusedCol = %d, want 0", m3.FocusedCol())
	}

	// e jumps to end of current word ("package" ends at col 6).
	m4 := send(m3, "e").(dtui.Model)
	if m4.FocusedCol() != 6 {
		t.Errorf("after e: focusedCol = %d, want 6", m4.FocusedCol())
	}
}

func TestCursor_skipsHunkHeader(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)
	// focusedLine 0 is the hunk header → cursor motions are no-ops.
	if m.FocusedLine() != 0 {
		t.Fatalf("setup focusedLine = %d, want 0", m.FocusedLine())
	}
	m2 := send(m, "l", "l", "w", "e").(dtui.Model)
	if m2.FocusedCol() != 0 {
		t.Errorf("focusedCol on hunk header should stay 0, got %d", m2.FocusedCol())
	}
}

func TestNavigation_n_noComments(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)
	before := m.FocusedLine()

	// n with no comments should not panic and position unchanged or wraps to 0
	m2 := send(m, "n").(dtui.Model)
	_ = m2.FocusedLine()
	_ = before
}

func TestQuit(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)
	_, cmd := m.Update(pressKey("q"))
	if cmd == nil {
		t.Fatal("q should return a quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q cmd returned %T, want tea.QuitMsg", msg)
	}
}

func TestHelp_toggle(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	m2 := send(m, "?").(dtui.Model)
	if m2.CurrentMode() != dtui.ModeHelp {
		t.Errorf("? did not enter ModeHelp: mode = %d", m2.CurrentMode())
	}

	view := viewText(m2)
	if !strings.Contains(view, "Key Bindings") {
		t.Error("help view should contain 'Key Bindings'")
	}

	m3 := send(m2, "esc").(dtui.Model)
	if m3.CurrentMode() != dtui.ModeNormal {
		t.Errorf("esc from help should return to ModeNormal: mode = %d", m3.CurrentMode())
	}
}

func TestHelp_q_closes(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)
	m2 := send(m, "?").(dtui.Model)
	m3 := send(m2, "q").(dtui.Model)
	if m3.CurrentMode() != dtui.ModeNormal {
		t.Errorf("q from help should close: mode = %d", m3.CurrentMode())
	}
}

func TestComment_nonCommentableFile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := makeSession()
	pd := &diff.ParsedDiff{
		Files: []diff.ParsedFile{
			{OldPath: "a/old.go", NewPath: "/dev/null", IsDelete: true},
		},
	}
	m := dtui.New(session, pd, "")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3 := send(m2, "c").(dtui.Model)

	if m3.CurrentMode() != dtui.ModeNormal {
		t.Errorf("c on non-commentable file should stay in ModeNormal: mode = %d", m3.CurrentMode())
	}
	if m3.StatusMsg() == "" {
		t.Error("c on non-commentable file should set a status message")
	}
}

func TestComment_enterAndCancel(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Navigate to a non-hunk-header line first (line 0 is hunk header)
	m2 := send(m, "j").(dtui.Model)
	m3 := send(m2, "c").(dtui.Model)
	if m3.CurrentMode() != dtui.ModeComment {
		t.Fatalf("c should enter ModeComment: mode = %d", m3.CurrentMode())
	}

	// Esc cancels
	m4 := send(m3, "esc").(dtui.Model)
	if m4.CurrentMode() != dtui.ModeNormal {
		t.Errorf("esc should return to ModeNormal: mode = %d", m4.CurrentMode())
	}
}

func TestComment_submitPersists(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	// Pre-create the session dir so SaveComments can write
	sessionDir := diff.SessionDir("testorg", "testrepo", diff.PRSessionKey(42))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := setupModel(t)
	// Navigate past the hunk header
	m2 := send(m, "j").(dtui.Model)
	// Enter comment mode
	m3 := send(m2, "c").(dtui.Model)
	if m3.CurrentMode() != dtui.ModeComment {
		t.Fatalf("c should enter ModeComment: mode = %d", m3.CurrentMode())
	}

	// Type comment body then submit
	for _, r := range "this is a test comment" {
		var tmp tea.Model
		tmp, _ = m3.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m3 = tmp.(dtui.Model)
	}
	m4 := send(m3, "ctrl+s").(dtui.Model)
	if m4.CurrentMode() != dtui.ModeNormal {
		t.Errorf("ctrl+s should return to ModeNormal: mode = %d", m4.CurrentMode())
	}

	// Verify comments.yaml was written
	comments, err := diff.LoadComments("testorg", "testrepo", diff.PRSessionKey(42))
	if err != nil {
		t.Fatalf("LoadComments: %v", err)
	}
	if len(comments) == 0 {
		t.Fatal("expected at least one comment in comments.yaml")
	}
	if !strings.Contains(comments[0].Body, "test comment") {
		t.Errorf("comment body = %q, want to contain 'test comment'", comments[0].Body)
	}
	if comments[0].File == "" {
		t.Error("comment file should not be empty")
	}
}

func TestOrphan_noOrphans(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	m2 := send(m, "o").(dtui.Model)
	// No orphans → status message, stays in ModeNormal
	if m2.CurrentMode() != dtui.ModeNormal {
		t.Errorf("o with no orphans should stay in ModeNormal: mode = %d", m2.CurrentMode())
	}
	if m2.StatusMsg() == "" {
		t.Error("o with no orphans should set a status message")
	}
}

func TestView_rendersHeader(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)
	view := viewText(m)
	if !strings.Contains(view, "sigil diff") {
		t.Error("view should contain 'sigil diff' in header")
	}
	if !strings.Contains(view, "PR #42") {
		t.Error("view should contain PR number")
	}
}

func TestView_rendersPRCommentsEntry(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)
	view := viewText(m)
	if !strings.Contains(view, "PR Comments") {
		t.Error("file list should always contain 'PR Comments' entry")
	}
}

func TestNavigation_ShiftTab_fromFile0_goesPRComments(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Shift-Tab from file 0 should go to PR Comments (index -1).
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m2m := m2.(dtui.Model)
	if m2m.FileIdx() != -1 {
		t.Errorf("Shift-Tab from file 0: fileIdx = %d, want -1 (PR Comments)", m2m.FileIdx())
	}
}

func TestNavigation_PRComments_view(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Navigate to PR Comments view via Shift-Tab.
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m2m := m2.(dtui.Model)
	if m2m.FileIdx() != -1 {
		t.Fatalf("expected fileIdx -1, got %d", m2m.FileIdx())
	}

	view := viewText(m2m)
	// The PR Comments view should show a prompt since there are no PR-level comments yet.
	if !strings.Contains(view, "No PR-level comments") {
		t.Error("PR Comments view should show 'No PR-level comments' when empty")
	}
}

func TestPRComment_c_in_PRCommentsView_entersModeComment(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Navigate to PR Comments view, then c should enter ModeComment.
	m1 := send(m, "shift+tab").(dtui.Model)
	m2 := send(m1, "c").(dtui.Model)
	if m2.CurrentMode() != dtui.ModeComment {
		t.Errorf("c in PR Comments view should enter ModeComment: mode = %d", m2.CurrentMode())
	}
}

func TestPRComment_submitPersists(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	sessionDir := diff.SessionDir("testorg", "testrepo", diff.PRSessionKey(42))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := setupModel(t)

	// Navigate to PR Comments view then press c to open modal.
	m1 := send(m, "shift+tab").(dtui.Model)
	m2 := send(m1, "c").(dtui.Model)
	if m2.CurrentMode() != dtui.ModeComment {
		t.Fatalf("c in PR Comments view should enter ModeComment: mode = %d", m2.CurrentMode())
	}

	// Type a PR-level comment body.
	for _, r := range "pr level comment here" {
		var tmp tea.Model
		tmp, _ = m2.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m2 = tmp.(dtui.Model)
	}

	// Submit with Ctrl+S.
	m3 := send(m2, "ctrl+s").(dtui.Model)
	if m3.CurrentMode() != dtui.ModeNormal {
		t.Errorf("ctrl+s should return to ModeNormal: mode = %d", m3.CurrentMode())
	}

	// Verify the comment was persisted with File == "".
	comments, err := diff.LoadComments("testorg", "testrepo", diff.PRSessionKey(42))
	if err != nil {
		t.Fatalf("LoadComments: %v", err)
	}
	if len(comments) == 0 {
		t.Fatal("expected at least one comment")
	}
	found := false
	for _, c := range comments {
		if c.File == "" && strings.Contains(c.Body, "pr level comment here") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a PR-level comment (File='') with the submitted body")
	}
}
