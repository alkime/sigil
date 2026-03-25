package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/alkime/sigil/cli"
	"github.com/alkime/sigil/parser"
)

// writeTempFile creates a temp markdown file with the given content and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

const testFileWithComments = `# Hello

<!-- @review-ref 0001 -->
Some content here
More content here

<!-- @review-ref 0002 -->
Another section

<!--
@review-backmatter

"0001":
  offset: 1
  span: 2
  comment: "Fix this paragraph"
  status: open

"0002":
  offset: 1
  span: 1
  comment: "Reword this"
  status: resolved

-->
`

const testFileNoComments = `# Hello

Just some plain markdown with no review comments.

Another paragraph.
`

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", "0001"},
		{"0001", "0001"},
		{"42", "0042"},
		{"abc", "0abc"},
		{"0", "0000"},
		{"999", "0999"},
		{"  1  ", "0001"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cli.NormalizeID(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestKongParsing(t *testing.T) {
	t.Run("empty args routes to TUI", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c,
			kong.Name("sigil"),
			kong.Exit(func(int) {}),
		)
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		// No args at all — should fail because TUI requires a file arg
		// but the default command should be selected.
		ctx, err := p.Parse([]string{"somefile.md"})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		cmd := ctx.Command()
		if cmd != "tui <file>" {
			t.Errorf("expected default TUI command, got %q", cmd)
		}
	})

	t.Run("get-comments routes to GetComments", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c, kong.Name("sigil"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		ctx, err := p.Parse([]string{"get-comments", "file.md"})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		cmd := ctx.Command()
		if cmd != "get-comments <file>" {
			t.Errorf("expected 'get-comments <file>', got %q", cmd)
		}
	})

	t.Run("subcommand missing required arg errors", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c, kong.Name("sigil"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		// get-comments requires a file argument; omitting it should error.
		_, err = p.Parse([]string{"get-comments"})
		if err == nil {
			t.Error("expected error for missing required arg, got nil")
		}
	})

	t.Run("no args errors", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c, kong.Name("sigil"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		// No args at all — TUI default requires a file arg.
		_, err = p.Parse([]string{})
		if err == nil {
			t.Error("expected error for no args, got nil")
		}
	})
}

// --- GetComments tests ---

func TestGetComments_All(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.GetCommentsCmd{File: path}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Comment 0001 [open]") {
		t.Errorf("expected open comment 0001 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Comment 0002 [resolved]") {
		t.Errorf("expected resolved comment 0002 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Fix this paragraph") {
		t.Errorf("expected comment text in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Reword this") {
		t.Errorf("expected comment text in output, got:\n%s", out)
	}
}

func TestGetComments_OpenFilter(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.GetCommentsCmd{File: path, Open: true}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Comment 0001 [open]") {
		t.Errorf("expected open comment 0001 in output, got:\n%s", out)
	}
	if strings.Contains(out, "Comment 0002") {
		t.Errorf("did not expect resolved comment 0002 in --open output, got:\n%s", out)
	}
}

func TestGetComments_ResolvedFilter(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.GetCommentsCmd{File: path, Resolved: true}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Comment 0001") {
		t.Errorf("did not expect open comment 0001 in --resolved output, got:\n%s", out)
	}
	if !strings.Contains(out, "Comment 0002 [resolved]") {
		t.Errorf("expected resolved comment 0002 in output, got:\n%s", out)
	}
}

func TestGetComments_NoComments(t *testing.T) {
	path := writeTempFile(t, testFileNoComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.GetCommentsCmd{File: path}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Comment") {
		t.Errorf("expected no comments in output, got:\n%s", out)
	}
}

func TestGetComments_SourceLines(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.GetCommentsCmd{File: path}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	// Comment 0001 has offset=1, span=2 and covers "Some content here" / "More content here"
	if !strings.Contains(out, "> Some content here") {
		t.Errorf("expected quoted source line 'Some content here' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "> More content here") {
		t.Errorf("expected quoted source line 'More content here' in output, got:\n%s", out)
	}
	// Comment 0002 covers "Another section"
	if !strings.Contains(out, "> Another section") {
		t.Errorf("expected quoted source line 'Another section' in output, got:\n%s", out)
	}
	// Verify line numbers are present (Lines: N-M format)
	if !strings.Contains(out, "Lines:") {
		t.Errorf("expected 'Lines:' header in output, got:\n%s", out)
	}
}

// --- ResolveComments tests ---

func TestResolveComments_Single(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.ResolveCommentsCmd{File: path, IDs: []string{"0001"}}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Resolved 0001") {
		t.Errorf("expected confirmation message, got: %s", buf.String())
	}
	// Re-parse and verify
	doc, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc.CommentByID["0001"] == nil {
		t.Fatal("comment 0001 not found after resolve")
	}
	if doc.CommentByID["0001"].Status != "resolved" {
		t.Errorf("expected status 'resolved', got %q", doc.CommentByID["0001"].Status)
	}
}

func TestResolveComments_Multiple(t *testing.T) {
	// Use a file where both are open initially.
	content := `# Test

<!-- @review-ref 0001 -->
Line A

<!-- @review-ref 0002 -->
Line B

<!--
@review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Comment A"
  status: open

"0002":
  offset: 1
  span: 1
  comment: "Comment B"
  status: open

-->
`
	path := writeTempFile(t, content)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.ResolveCommentsCmd{File: path, IDs: []string{"0001", "0002"}}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	doc, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	for _, id := range []string{"0001", "0002"} {
		c := doc.CommentByID[id]
		if c == nil {
			t.Fatalf("comment %s not found after resolve", id)
		}
		if c.Status != "resolved" {
			t.Errorf("comment %s: expected status 'resolved', got %q", id, c.Status)
		}
	}
}

func TestResolveComments_NotFound(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.ResolveCommentsCmd{File: path, IDs: []string{"9999"}}
	err := cmd.Run(ctx)
	if err == nil {
		t.Fatal("expected error for nonexistent comment, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestResolveComments_NormalizedID(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	// Pass "1" instead of "0001" — should normalize and resolve.
	cmd := &cli.ResolveCommentsCmd{File: path, IDs: []string{"1"}}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	doc, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc.CommentByID["0001"].Status != "resolved" {
		t.Errorf("expected status 'resolved' after normalizing '1' -> '0001', got %q",
			doc.CommentByID["0001"].Status)
	}
}

// --- UnresolveComments tests ---

func TestUnresolveComments(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	// Comment 0002 is resolved; unresolve it.
	cmd := &cli.UnresolveCommentsCmd{File: path, IDs: []string{"0002"}}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Unresolved 0002") {
		t.Errorf("expected confirmation message, got: %s", buf.String())
	}
	doc, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc.CommentByID["0002"].Status != "open" {
		t.Errorf("expected status 'open' after unresolve, got %q", doc.CommentByID["0002"].Status)
	}
}

// --- ReplyComment tests ---

func TestReplyComment(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.ReplyCommentCmd{File: path, ID: "0001", ReplyText: "I fixed this."}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Replied to 0001") {
		t.Errorf("expected confirmation message, got: %s", buf.String())
	}
	doc, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	c := doc.CommentByID["0001"]
	if c == nil {
		t.Fatal("comment 0001 not found after reply")
	}
	if !strings.Contains(c.Comment, "REPLY: I fixed this.") {
		t.Errorf("expected reply text in comment, got: %q", c.Comment)
	}
	if !strings.Contains(c.Comment, "Fix this paragraph") {
		t.Errorf("expected original text preserved, got: %q", c.Comment)
	}
}

func TestReplyComment_PreservesStatus(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	// Reply to resolved comment 0002 — status should stay resolved.
	cmd := &cli.ReplyCommentCmd{File: path, ID: "0002", ReplyText: "Acknowledged."}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	doc, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	c := doc.CommentByID["0002"]
	if c == nil {
		t.Fatal("comment 0002 not found after reply")
	}
	if c.Status != "resolved" {
		t.Errorf("expected status to remain 'resolved', got %q", c.Status)
	}
	if !strings.Contains(c.Comment, "REPLY: Acknowledged.") {
		t.Errorf("expected reply text in comment, got: %q", c.Comment)
	}
}

func TestReplyComment_NotFound(t *testing.T) {
	path := writeTempFile(t, testFileWithComments)
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.ReplyCommentCmd{File: path, ID: "9999", ReplyText: "Hello"}
	err := cmd.Run(ctx)
	if err == nil {
		t.Fatal("expected error for nonexistent comment, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// --- GenerateSkill tests ---

func TestGenerateSkill(t *testing.T) {
	var buf bytes.Buffer
	ctx := &cli.CLIContext{Out: &buf}
	cmd := &cli.GenerateSkillCmd{}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	// Check for key strings that should be in the skill content.
	for _, want := range []string{
		"Sigil",
		"get-comments",
		"resolve-comments",
		"reply-comment",
		"generate-skill",
		"@review-backmatter",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected skill output to contain %q", want)
		}
	}
}
