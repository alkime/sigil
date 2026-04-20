package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alkime/sigil/cli"
	"github.com/alkime/sigil/diff"
	"gopkg.in/yaml.v3"
)

const fixtureSessionID = "aaaabbbb-cccc-4ddd-eeee-ffffffffffff"
const fixtureCommentID = "11112222-3333-4444-5555-666677778888"
const fixtureCommentID2 = "aaaabbbb-1111-2222-3333-444455556666"

// seedFixture creates a minimal session + comments on disk and returns the XDG data dir.
func seedFixture(t *testing.T) string {
	t.Helper()
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	sessionDir := filepath.Join(xdgDir, "sigil", "diffs", "testorg", "testrepo", "42")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	session := diff.Session{
		ID:         fixtureSessionID,
		Repo:       "testorg/testrepo",
		PRNumber:   42,
		PRTitle:    "My Feature",
		BaseBranch: "main",
		BaseSHA:    "baaaaase",
		HeadSHA:    "heeead1",
		Branch:     "feature/my-pr",
		CreatedAt:  now,
		UpdatedAt:  now,
		Snapshots: []diff.Snapshot{
			{Base: "baaaaase", Head: "heeead1", ObservedAt: now},
		},
	}
	sessionBytes, _ := yaml.Marshal(session)
	if err := os.WriteFile(filepath.Join(sessionDir, "session.yaml"), sessionBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	comments := []diff.Comment{
		{
			ID:       fixtureCommentID,
			File:     "internal/auth/oidc.go",
			HunkHeader: "@@ -42,7 +42,12 @@",
			LineHint: 47,
			Side:     "right",
			Context: diff.CommentContext{
				Before: []string{"  if err != nil {", "    return nil, err"},
				Target: "  return token, nil",
				After:  []string{"}", ""},
			},
			Body:      "should we be logging the token type here?",
			Author:    "james",
			CreatedAt: now,
			UpdatedAt: now,
			Resolved:  false,
		},
		{
			ID:       fixtureCommentID2,
			File:     "internal/auth/token.go",
			HunkHeader: "@@ -10,5 +10,6 @@",
			LineHint: 12,
			Side:     "right",
			Context: diff.CommentContext{
				Before: []string{"func New() *Token {"},
				Target: "  return &Token{}",
				After:  []string{"}"},
			},
			Body:      "add a comment explaining Token fields",
			Author:    "james",
			CreatedAt: now,
			UpdatedAt: now,
			Resolved:  true,
		},
	}
	commentsBytes, _ := yaml.Marshal(comments)
	if err := os.WriteFile(filepath.Join(sessionDir, "comments.yaml"), commentsBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	return xdgDir
}

func diffCtx(t *testing.T, sessionID string) (*bytes.Buffer, *cli.CLIContext) {
	t.Helper()
	buf := &bytes.Buffer{}
	ctx := &cli.CLIContext{Out: buf, DiffSession: sessionID}
	return buf, ctx
}

// --- DiffGetCommentsCmd ---

func TestDiffGetComments_all(t *testing.T) {
	seedFixture(t)
	buf, ctx := diffCtx(t, fixtureSessionID)

	cmd := &cli.DiffGetCommentsCmd{}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Comment "+fixtureCommentID+" [open]") {
		t.Errorf("expected open comment in output:\n%s", out)
	}
	if !strings.Contains(out, "Comment "+fixtureCommentID2+" [resolved]") {
		t.Errorf("expected resolved comment in output:\n%s", out)
	}
	if !strings.Contains(out, "File: internal/auth/oidc.go") {
		t.Errorf("expected file path in output:\n%s", out)
	}
	if !strings.Contains(out, "Hunk target: `  return token, nil`") {
		t.Errorf("expected hunk target in output:\n%s", out)
	}
	if !strings.Contains(out, "should we be logging the token type here?") {
		t.Errorf("expected comment body in output:\n%s", out)
	}
	if !strings.Contains(out, "> @@ -42,7 +42,12 @@") {
		t.Errorf("expected hunk header in output:\n%s", out)
	}
	if !strings.Contains(out, ">   return token, nil") {
		t.Errorf("expected target line in output:\n%s", out)
	}
}

func TestDiffGetComments_openFilter(t *testing.T) {
	seedFixture(t)
	buf, ctx := diffCtx(t, fixtureSessionID)

	cmd := &cli.DiffGetCommentsCmd{Open: true}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, fixtureCommentID) {
		t.Errorf("expected open comment in --open output:\n%s", out)
	}
	if strings.Contains(out, fixtureCommentID2) {
		t.Errorf("did not expect resolved comment in --open output:\n%s", out)
	}
}

func TestDiffGetComments_resolvedFilter(t *testing.T) {
	seedFixture(t)
	buf, ctx := diffCtx(t, fixtureSessionID)

	cmd := &cli.DiffGetCommentsCmd{Resolved: true}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, fixtureCommentID) {
		t.Errorf("did not expect open comment in --resolved output:\n%s", out)
	}
	if !strings.Contains(out, fixtureCommentID2) {
		t.Errorf("expected resolved comment in --resolved output:\n%s", out)
	}
}

// --- DiffReplyCommentCmd ---

func TestDiffReplyComment(t *testing.T) {
	xdgDir := seedFixture(t)
	buf, ctx := diffCtx(t, fixtureSessionID)

	cmd := &cli.DiffReplyCommentCmd{
		ID:        fixtureCommentID,
		ReplyText: "I fixed this.",
	}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Replied to "+fixtureCommentID) {
		t.Errorf("expected confirmation, got: %s", buf.String())
	}

	// Verify on-disk state.
	commentsPath := filepath.Join(xdgDir, "sigil", "diffs", "testorg", "testrepo", "42", "comments.yaml")
	data, err := os.ReadFile(commentsPath)
	if err != nil {
		t.Fatalf("read comments.yaml: %v", err)
	}
	var comments []diff.Comment
	if err := yaml.Unmarshal(data, &comments); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var found *diff.Comment
	for i := range comments {
		if comments[i].ID == fixtureCommentID {
			found = &comments[i]
			break
		}
	}
	if found == nil {
		t.Fatal("comment not found after reply")
	}
	if !strings.Contains(found.Body, "REPLY: I fixed this.") {
		t.Errorf("expected REPLY in body, got: %q", found.Body)
	}
	if !strings.Contains(found.Body, "should we be logging") {
		t.Errorf("expected original body preserved, got: %q", found.Body)
	}
}

func TestDiffReplyComment_notFound(t *testing.T) {
	seedFixture(t)
	_, ctx := diffCtx(t, fixtureSessionID)

	cmd := &cli.DiffReplyCommentCmd{ID: "nonexistent-id", ReplyText: "hello"}
	err := cmd.Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- DiffResolveCmd ---

func TestDiffResolveComment(t *testing.T) {
	xdgDir := seedFixture(t)
	buf, ctx := diffCtx(t, fixtureSessionID)

	cmd := &cli.DiffResolveCmd{IDs: []string{fixtureCommentID}}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Resolved "+fixtureCommentID) {
		t.Errorf("expected confirmation, got: %s", buf.String())
	}

	commentsPath := filepath.Join(xdgDir, "sigil", "diffs", "testorg", "testrepo", "42", "comments.yaml")
	var comments []diff.Comment
	data, _ := os.ReadFile(commentsPath)
	_ = yaml.Unmarshal(data, &comments)

	for _, c := range comments {
		if c.ID == fixtureCommentID && !c.Resolved {
			t.Errorf("expected comment to be resolved, but Resolved=false")
		}
	}
}

// --- DiffUnresolveCmd ---

func TestDiffUnresolveComment(t *testing.T) {
	xdgDir := seedFixture(t)
	buf, ctx := diffCtx(t, fixtureSessionID)

	// fixtureCommentID2 starts as resolved=true
	cmd := &cli.DiffUnresolveCmd{IDs: []string{fixtureCommentID2}}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Unresolved "+fixtureCommentID2) {
		t.Errorf("expected confirmation, got: %s", buf.String())
	}

	commentsPath := filepath.Join(xdgDir, "sigil", "diffs", "testorg", "testrepo", "42", "comments.yaml")
	var comments []diff.Comment
	data, _ := os.ReadFile(commentsPath)
	_ = yaml.Unmarshal(data, &comments)

	for _, c := range comments {
		if c.ID == fixtureCommentID2 && c.Resolved {
			t.Errorf("expected comment to be unresolved, but Resolved=true")
		}
	}
}

func TestDiffResolveComment_notFound(t *testing.T) {
	seedFixture(t)
	_, ctx := diffCtx(t, fixtureSessionID)

	cmd := &cli.DiffResolveCmd{IDs: []string{"nonexistent-id"}}
	err := cmd.Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}
