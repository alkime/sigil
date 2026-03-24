package writer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alkime/sigil/parser"
)

func TestWriteComment_NoExistingComments(t *testing.T) {
	content := "# Title\n\nSome content here.\n\nMore content.\n"
	tmp := writeTempFile(t, content)

	doc, err := parser.Parse(tmp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Add a comment on "Some content here." (raw line 2)
	newDoc, err := WriteComment(doc, 2, 1, "This needs work")
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if len(newDoc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(newDoc.Comments))
	}
	c := newDoc.Comments[0]
	if c.ID != "0001" {
		t.Errorf("expected ID 0001, got %s", c.ID)
	}
	if c.Comment != "This needs work" {
		t.Errorf("unexpected comment: %s", c.Comment)
	}
	if c.Span != 1 {
		t.Errorf("expected span 1, got %d", c.Span)
	}
	if len(newDoc.RefMarkers) != 1 {
		t.Errorf("expected 1 ref marker, got %d", len(newDoc.RefMarkers))
	}
}

func TestWriteComment_WithExistingComments(t *testing.T) {
	content := `# Title

<!-- @review-ref 0001 -->
First paragraph.

Second paragraph.

<!--
@review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Existing comment"
  status: open
-->
`
	tmp := writeTempFile(t, content)

	doc, err := parser.Parse(tmp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 existing comment, got %d", len(doc.Comments))
	}

	// Add a comment on "Second paragraph." (raw line 5)
	newDoc, err := WriteComment(doc, 5, 1, "New comment here")
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if len(newDoc.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(newDoc.Comments))
	}

	// Check IDs
	ids := map[string]bool{}
	for _, c := range newDoc.Comments {
		ids[c.ID] = true
	}
	if !ids["0001"] || !ids["0002"] {
		t.Errorf("expected IDs 0001 and 0002, got %v", ids)
	}

	// Verify original comment survived
	if c, ok := newDoc.CommentByID["0001"]; !ok || c.Comment != "Existing comment" {
		t.Error("original comment was lost or modified")
	}
}

func TestWriteComment_RoundTrip(t *testing.T) {
	content := "# Doc\n\nLine A\n\nLine B\n\nLine C\n"
	tmp := writeTempFile(t, content)

	doc, err := parser.Parse(tmp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Add first comment
	doc, err = WriteComment(doc, 2, 1, "Comment on A")
	if err != nil {
		t.Fatalf("write 1: %v", err)
	}

	// Add second comment
	doc, err = WriteComment(doc, 5, 1, "Comment on B")
	if err != nil {
		t.Fatalf("write 2: %v", err)
	}

	// Re-read and verify
	final, err := parser.Parse(tmp)
	if err != nil {
		t.Fatalf("final parse: %v", err)
	}

	if len(final.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(final.Comments))
	}
	if len(final.RefMarkers) != 2 {
		t.Fatalf("expected 2 ref markers, got %d", len(final.RefMarkers))
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "test.md")
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return tmp
}
