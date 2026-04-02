package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/alkime/sigil/model"
	"github.com/alkime/sigil/parser"
	"github.com/alkime/sigil/writer"
	"github.com/charmbracelet/x/ansi"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupApp writes content to a temp file, parses it, creates an AppModel with
// real writer callbacks, and sends a WindowSizeMsg to initialize the viewport.
func setupApp(t *testing.T, content string) (tea.Model, string) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "test.md")
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	doc, err := parser.Parse(tmp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	app := model.NewApp(doc, writer.WriteComment, writer.UpdateComment, writer.DeleteComment)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m, tmp
}

// setupAppFromFile copies a file to a temp dir, parses it, and returns the model.
func setupAppFromFile(t *testing.T, srcPath string) (tea.Model, string) {
	t.Helper()
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read %s: %v", srcPath, err)
	}
	return setupApp(t, string(data))
}

// key constructs a tea.KeyPressMsg that matches how the app switches on msg.String().
func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+a":
		return tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl}
	default:
		if len(s) == 1 {
			r := rune(s[0])
			return tea.KeyPressMsg{Code: r, Text: s}
		}
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
}

// send chains Update() calls for each key string and returns the final model.
func send(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		m, _ = m.Update(key(k))
	}
	return m
}

// sendCmd is like send but also returns the last tea.Cmd.
func sendCmd(m tea.Model, keys ...string) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = m.Update(key(k))
	}
	return m, cmd
}

// typeText sends individual rune key presses for each character in text.
func typeText(m tea.Model, text string) tea.Model {
	for _, r := range text {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// viewText returns View().Content with ANSI escape codes stripped.
func viewText(m tea.Model) string {
	return ansi.Strip(m.(model.AppModel).View().Content)
}

// viewHasGutterMarker checks if any line has the commented-block gutter marker.
func viewHasGutterMarker(m tea.Model) bool {
	vt := viewText(m)
	return strings.Contains(vt, "●") || strings.Contains(vt, "▶")
}

// ---------------------------------------------------------------------------
// Test data
//
// These documents use multiple headings to provide good anchor points for
// the content-to-rendered line mapping (buildLineMapping anchors on headings).
// ---------------------------------------------------------------------------

const testDocPlain = `# Overview

First paragraph of the document.

## Details

Second paragraph with more details.

## Conclusion

Third paragraph wrapping up.
`

// Uses the real testdata/sample.md structure — proven to map correctly.
const testDocOneOpen = `# Architecture Design

<!-- @review-ref 0001 -->
The system uses a simple token-based auth flow.

## Database Schema

We use a single users table with no indexes.

## Deployment

Standard Docker-based deployment.

<!--
@review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Needs work"
  status: open
-->
`

const testDocOneResolved = `# Architecture Design

<!-- @review-ref 0001 -->
The system uses a simple token-based auth flow.

## Database Schema

We use a single users table with no indexes.

## Deployment

Standard Docker-based deployment.

<!--
@review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Needs work"
  status: resolved
-->
`

const testDocTwoComments = `# Architecture Design

<!-- @review-ref 0001 -->
The system uses a simple token-based auth flow.

## Database Schema

<!-- @review-ref 0002 -->
We use a single users table with no indexes.

## Deployment

Standard Docker-based deployment.

<!--
@review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Fix first"
  status: open

"0002":
  offset: 1
  span: 1
  comment: "Fix second"
  status: open
-->
`

// ---------------------------------------------------------------------------
// Group 1: State machine basics
// ---------------------------------------------------------------------------

func TestQuit(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)
	_, cmd := sendCmd(m, "q")
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}
}

func TestQuitCtrlC(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)
	_, cmd := sendCmd(m, "ctrl+c")
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}
}

func TestHelpToggle(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)

	m = send(m, "?")
	vt := viewText(m)
	if !strings.Contains(vt, "Keybindings") {
		t.Error("expected help view to contain 'Keybindings'")
	}

	// Toggle back
	m = send(m, "?")
	vt = viewText(m)
	if strings.Contains(vt, "Keybindings") {
		t.Error("expected browse view to not contain 'Keybindings'")
	}
}

func TestHelpCloseEsc(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)
	m = send(m, "?", "esc")
	vt := viewText(m)
	if strings.Contains(vt, "Keybindings") {
		t.Error("expected browse view after esc from help")
	}
}

// ---------------------------------------------------------------------------
// Group 2: Navigation
// ---------------------------------------------------------------------------

func TestNavigateJK(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)

	v1 := viewText(m)
	m = send(m, "j", "j")
	v2 := viewText(m)
	m = send(m, "k", "k")
	v3 := viewText(m)

	// The gutter changes with cursor position, so views should differ
	if v1 == v2 {
		t.Error("expected view to change after j,j navigation")
	}
	if v1 != v3 {
		t.Error("expected view to return to original after k,k")
	}
}

func TestJumpToComment(t *testing.T) {
	m, _ := setupApp(t, testDocTwoComments)

	m = send(m, "n")
	if !viewHasGutterMarker(m) {
		t.Error("expected n to land on a commented block (gutter marker missing)")
	}
}

func TestJumpPrevComment(t *testing.T) {
	m, _ := setupApp(t, testDocTwoComments)

	m = send(m, "N")
	if !viewHasGutterMarker(m) {
		t.Error("expected N to land on a commented block (gutter marker missing)")
	}
}

func TestGotoTopBottom(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)

	v1 := viewText(m)
	m = send(m, "G")
	vBottom := viewText(m)
	m = send(m, "g")
	vTop := viewText(m)

	// After G and g, we should be back at the same view as start
	if vTop != v1 {
		t.Error("expected g to return to top")
	}
	// G should change the gutter position
	if vBottom == v1 {
		t.Error("expected G to change view")
	}
}

// ---------------------------------------------------------------------------
// Group 3: Comment CRUD (using real testdata/sample.md for reliable mapping)
// ---------------------------------------------------------------------------

func TestCreateComment(t *testing.T) {
	m, tmpPath := setupApp(t, testDocPlain)

	// Enter on first block opens create modal
	m = send(m, "enter")
	vt := viewText(m)
	if !strings.Contains(vt, "New Comment") {
		t.Fatal("expected create modal with 'New Comment'")
	}

	// Type comment text and submit
	m = typeText(m, "hello")
	m = send(m, "ctrl+s")

	// Should be back in browse (no modal)
	vt = viewText(m)
	if strings.Contains(vt, "New Comment") {
		t.Error("expected modal to close after submit")
	}

	// Verify on disk
	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(doc.Comments))
	}
	if doc.Comments[0].Comment != "hello" {
		t.Errorf("expected comment text 'hello', got %q", doc.Comments[0].Comment)
	}
}

func TestCreateCommentCancel(t *testing.T) {
	m, tmpPath := setupApp(t, testDocPlain)

	m = send(m, "enter")
	m = typeText(m, "this should be discarded")
	m = send(m, "esc")

	vt := viewText(m)
	if strings.Contains(vt, "New Comment") {
		t.Error("expected modal to close after esc")
	}

	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(doc.Comments) != 0 {
		t.Errorf("expected 0 comments after cancel, got %d", len(doc.Comments))
	}
}

func TestResolveToggle(t *testing.T) {
	m, tmpPath := setupAppFromFile(t, "../testdata/sample.md")

	// Navigate to a comment
	m = send(m, "n")
	if !viewHasGutterMarker(m) {
		t.Fatal("n did not navigate to commented block")
	}

	// Resolve
	m = send(m, "r")
	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc.CommentByID["0001"].Status != "resolved" {
		t.Errorf("expected 'resolved', got %q", doc.CommentByID["0001"].Status)
	}

	// Reopen
	m = send(m, "r")
	doc, err = parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc.CommentByID["0001"].Status != "open" {
		t.Errorf("expected 'open', got %q", doc.CommentByID["0001"].Status)
	}
}

func TestEditComment(t *testing.T) {
	m, tmpPath := setupAppFromFile(t, "../testdata/sample.md")

	// Navigate to comment and open inspect
	m = send(m, "n", "enter")
	vt := viewText(m)
	if !strings.Contains(vt, "0001") {
		t.Fatal("expected inspect modal to show comment ID")
	}

	// Select all and replace
	m, _ = m.Update(key("ctrl+a"))
	m = typeText(m, "updated text")
	m = send(m, "ctrl+s")

	vt = viewText(m)
	if strings.Contains(vt, "Comment [") {
		t.Error("expected modal to close after save")
	}

	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	c := doc.CommentByID["0001"]
	if c == nil {
		t.Fatal("comment 0001 not found after edit")
	}
	if !strings.Contains(c.Comment, "updated text") {
		t.Errorf("expected comment to contain 'updated text', got %q", c.Comment)
	}
}

func TestDeleteResolved(t *testing.T) {
	m, tmpPath := setupAppFromFile(t, "../testdata/sample.md")

	// First resolve the comment, then delete it
	m = send(m, "n", "r")

	// Now delete
	m = send(m, "d")
	vt := viewText(m)
	if !strings.Contains(vt, "Delete this comment permanently") {
		t.Fatal("expected delete confirmation prompt")
	}

	// Confirm
	m = send(m, "y")
	vt = viewText(m)
	if strings.Contains(vt, "Delete this comment permanently") {
		t.Error("expected confirmation to close after y")
	}

	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc.CommentByID["0001"] != nil {
		t.Error("expected comment 0001 to be deleted")
	}
}

func TestDeleteOpenNoop(t *testing.T) {
	m, tmpPath := setupAppFromFile(t, "../testdata/sample.md")

	m = send(m, "n", "d")
	vt := viewText(m)
	// d on open comment should be a no-op — no confirm dialog
	if strings.Contains(vt, "Delete this comment permanently") {
		t.Error("expected no delete prompt on open comment")
	}

	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(doc.Comments) != 3 {
		t.Errorf("expected 3 comments unchanged, got %d", len(doc.Comments))
	}
}

func TestDeleteCancel(t *testing.T) {
	m, tmpPath := setupAppFromFile(t, "../testdata/sample.md")

	// Resolve first, then try delete but cancel
	m = send(m, "n", "r", "d", "n")
	vt := viewText(m)
	if strings.Contains(vt, "Delete this comment permanently") {
		t.Error("expected confirmation to close after n (cancel)")
	}

	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc.CommentByID["0001"] == nil {
		t.Error("expected comment 0001 to still exist after cancel")
	}
}

// ---------------------------------------------------------------------------
// Group 4: Multi-select
// ---------------------------------------------------------------------------

func TestMultiSelectComment(t *testing.T) {
	m, tmpPath := setupApp(t, testDocPlain)

	// Start selection, extend, then enter to create comment
	m = send(m, "x", "j", "enter")
	vt := viewText(m)
	if !strings.Contains(vt, "New Comment") {
		t.Fatal("expected create modal for multi-select")
	}

	m = typeText(m, "range comment")
	m = send(m, "ctrl+s")

	doc, err := parser.Parse(tmpPath)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(doc.Comments))
	}
	if doc.Comments[0].Span <= 1 {
		t.Errorf("expected span > 1 for multi-select, got %d", doc.Comments[0].Span)
	}
}

func TestEscCancelsSelection(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)

	m = send(m, "x", "esc")
	vt := viewText(m)
	if strings.Contains(vt, "New Comment") {
		t.Error("expected no modal after esc from selection")
	}
}

// ---------------------------------------------------------------------------
// Group 5: View output sanity
// ---------------------------------------------------------------------------

func TestViewShowsContent(t *testing.T) {
	m, _ := setupApp(t, testDocPlain)
	vt := viewText(m)
	if !strings.Contains(vt, "Overview") {
		t.Error("expected rendered view to contain document title")
	}
}

func TestViewLoading(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.md")
	if err := os.WriteFile(tmp, []byte(testDocPlain), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	doc, err := parser.Parse(tmp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	app := model.NewApp(doc, writer.WriteComment, writer.UpdateComment, writer.DeleteComment)
	vt := ansi.Strip(app.View().Content)
	if !strings.Contains(vt, "Loading") {
		t.Error("expected 'Loading...' before WindowSizeMsg")
	}
}

func TestInspectModalShowsCommentID(t *testing.T) {
	m, _ := setupAppFromFile(t, "../testdata/sample.md")
	m = send(m, "n", "enter")
	vt := viewText(m)
	if !strings.Contains(vt, "0001") {
		t.Error("expected inspect modal to show comment ID '0001'")
	}
}

func TestDeleteConfirmShowsPrompt(t *testing.T) {
	m, _ := setupAppFromFile(t, "../testdata/sample.md")
	// Resolve first, then d
	m = send(m, "n", "r", "d")
	vt := viewText(m)
	if !strings.Contains(vt, "Delete this comment permanently") {
		t.Error("expected delete confirm to show prompt text")
	}
}
