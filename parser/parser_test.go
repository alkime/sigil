package parser

import (
	"testing"
)

func TestParseContent_NoComments(t *testing.T) {
	input := `# Hello World

Some content here.
`
	doc, err := ParseContent("test.md", []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(doc.Comments))
	}
	if len(doc.RefMarkers) != 0 {
		t.Errorf("expected 0 ref markers, got %d", len(doc.RefMarkers))
	}
	// ContentLines should equal RawLines (nothing stripped)
	if len(doc.ContentLines) != len(doc.RawLines) {
		t.Errorf("expected %d content lines, got %d", len(doc.RawLines), len(doc.ContentLines))
	}
}

func TestParseContent_SingleComment(t *testing.T) {
	input := `# Title

<!-- @review-ref 0001 -->
The auth flow is simple.
It uses tokens.

## Next Section

<!--
@review-backmatter

"0001":
  offset: 1
  span: 2
  comment: "Needs more detail on the auth flow."
  status: open
-->
`
	doc, err := ParseContent("test.md", []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(doc.Comments))
	}
	c := doc.Comments[0]
	if c.ID != "0001" {
		t.Errorf("expected ID 0001, got %s", c.ID)
	}
	if c.Offset != 1 {
		t.Errorf("expected offset 1, got %d", c.Offset)
	}
	if c.Span != 2 {
		t.Errorf("expected span 2, got %d", c.Span)
	}
	if c.Comment != "Needs more detail on the auth flow." {
		t.Errorf("unexpected comment text: %s", c.Comment)
	}
	if c.Status != "open" {
		t.Errorf("expected status open, got %s", c.Status)
	}

	if len(doc.RefMarkers) != 1 {
		t.Fatalf("expected 1 ref marker, got %d", len(doc.RefMarkers))
	}

	// ContentLines should not contain the ref marker or backmatter
	for _, line := range doc.ContentLines {
		if refMarkerRe.MatchString(line) {
			t.Errorf("content lines should not contain ref markers: %q", line)
		}
		if line == "@review-backmatter" {
			t.Errorf("content lines should not contain backmatter tag")
		}
	}

	// Check that the right content lines are marked as commented
	if len(doc.CommentedContentLines) == 0 {
		t.Fatal("expected some commented content lines")
	}

	// The ref marker is at raw line 2, offset=1, span=2
	// So raw lines 3 and 4 ("The auth flow..." and "It uses tokens.") are commented.
	// After stripping the ref marker (raw line 2), these become content lines 2 and 3.
	foundCommented := false
	for ci, ids := range doc.CommentedContentLines {
		if len(ids) > 0 && ids[0] == "0001" {
			foundCommented = true
			line := doc.ContentLines[ci]
			if line != "The auth flow is simple." && line != "It uses tokens." {
				t.Errorf("unexpected commented line: %q", line)
			}
		}
	}
	if !foundCommented {
		t.Error("did not find commented content lines for 0001")
	}
}

func TestParseContent_MultipleComments(t *testing.T) {
	input := `# Doc

<!-- @review-ref 0001 -->
Line A

<!-- @review-ref 0002 -->
Line B
Line C

<!-- @review-ref 0003 -->
Line D

<!--
@review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Comment on A"
  status: open

"0002":
  offset: 1
  span: 2
  comment: "Comment on B and C"
  status: resolved

"0003":
  offset: 1
  span: 1
  comment: "Comment on D"
  status: open
-->
`
	doc, err := ParseContent("test.md", []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(doc.Comments))
	}
	if len(doc.RefMarkers) != 3 {
		t.Fatalf("expected 3 ref markers, got %d", len(doc.RefMarkers))
	}

	// Verify CommentByID
	for _, id := range []string{"0001", "0002", "0003"} {
		if _, ok := doc.CommentByID[id]; !ok {
			t.Errorf("missing comment for ID %s", id)
		}
	}

	// Check resolved status
	if doc.CommentByID["0002"].Status != "resolved" {
		t.Errorf("expected comment 0002 to be resolved")
	}
}

func TestParseContent_BareIntegerKeys(t *testing.T) {
	// Test that bare integer YAML keys (0001 parsed as 1) are handled
	input := `# Doc

<!-- @review-ref 0001 -->
Content line.

<!--
@review-backmatter

1:
  offset: 1
  span: 1
  comment: "Test bare key"
  status: open
-->
`
	doc, err := ParseContent("test.md", []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(doc.Comments))
	}
	if doc.Comments[0].ID != "0001" {
		t.Errorf("expected ID 0001 (zero-padded), got %s", doc.Comments[0].ID)
	}

	// Should still map correctly to the ref marker
	if len(doc.CommentedContentLines) == 0 {
		t.Error("expected commented content lines from bare integer key")
	}
}

func TestParseContent_EmptyFile(t *testing.T) {
	doc, err := ParseContent("test.md", []byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(doc.Comments))
	}
	if len(doc.ContentLines) != 0 {
		t.Errorf("expected 0 content lines, got %d", len(doc.ContentLines))
	}
}

func TestParseContent_ContentToSourceMapping(t *testing.T) {
	input := `Line 0
<!-- @review-ref 0001 -->
Line 2
Line 3
`
	doc, err := ParseContent("test.md", []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Raw lines: 0="Line 0", 1="<!-- @review-ref 0001 -->", 2="Line 2", 3="Line 3"
	// Content lines: 0="Line 0" (source 0), 1="Line 2" (source 2), 2="Line 3" (source 3)
	if len(doc.ContentLines) != 3 {
		t.Fatalf("expected 3 content lines, got %d", len(doc.ContentLines))
	}

	expected := []struct {
		content string
		source  int
	}{
		{"Line 0", 0},
		{"Line 2", 2},
		{"Line 3", 3},
	}

	for i, exp := range expected {
		if doc.ContentLines[i] != exp.content {
			t.Errorf("content line %d: expected %q, got %q", i, exp.content, doc.ContentLines[i])
		}
		if doc.ContentToSource[i] != exp.source {
			t.Errorf("content-to-source %d: expected %d, got %d", i, exp.source, doc.ContentToSource[i])
		}
	}
}

func TestParseContent_CombinedBackmatterStart(t *testing.T) {
	// Test the format where <!-- and @review-backmatter are on the same line
	input := `# Doc

Content here.

<!-- @review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Test"
  status: open
-->
`
	doc, err := ParseContent("test.md", []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(doc.Comments))
	}
	if doc.Comments[0].Comment != "Test" {
		t.Errorf("unexpected comment: %s", doc.Comments[0].Comment)
	}
}
