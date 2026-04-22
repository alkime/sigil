package diff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReviewOrder_MissingFile(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadReviewOrder(dir)
	if err != nil {
		t.Fatalf("expected nil err on missing file, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil ReviewOrder on missing file, got %+v", got)
	}
}

func TestLoadReviewOrder_EmptyWorktree(t *testing.T) {
	got, err := LoadReviewOrder("")
	if err != nil {
		t.Fatalf("expected nil err on empty worktree, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil ReviewOrder on empty worktree, got %+v", got)
	}
}

func TestLoadReviewOrder_Malformed(t *testing.T) {
	dir := t.TempDir()
	writeOrderFile(t, dir, "files: [this is not valid yaml")
	_, err := LoadReviewOrder(dir)
	if err == nil {
		t.Fatalf("expected error on malformed yaml, got nil")
	}
}

func TestLoadReviewOrder_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeOrderFile(t, dir, `files:
  - path: a.go
    note: look here first
  - path: b.go
  - path: c.go
    note: last
`)
	got, err := LoadReviewOrder(dir)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil ReviewOrder")
	}
	if len(got.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got.Entries))
	}
	if got.Entries[0].Path != "a.go" || got.Entries[0].Note != "look here first" {
		t.Errorf("entry[0] = %+v", got.Entries[0])
	}
	if got.Entries[1].Path != "b.go" || got.Entries[1].Note != "" {
		t.Errorf("entry[1] = %+v", got.Entries[1])
	}
}

func TestReorder_AllMatched(t *testing.T) {
	r := &ReviewOrder{Entries: []ReviewOrderEntry{
		{Path: "b.go"}, {Path: "a.go"}, {Path: "c.go"},
	}}
	files := []ParsedFile{
		{NewPath: "a.go"},
		{NewPath: "b.go"},
		{NewPath: "c.go"},
	}
	out, dropped := r.Reorder(files)
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
	got := paths(out)
	want := []string{"b.go", "a.go", "c.go"}
	if !equal(got, want) {
		t.Errorf("Reorder = %v, want %v", got, want)
	}
}

func TestReorder_PartialMatch(t *testing.T) {
	// r lists a.go, b.go; PR has a.go, c.go → output is [a.go, c.go], dropped=1.
	r := &ReviewOrder{Entries: []ReviewOrderEntry{
		{Path: "a.go"}, {Path: "b.go"},
	}}
	files := []ParsedFile{
		{NewPath: "a.go"},
		{NewPath: "c.go"},
	}
	out, dropped := r.Reorder(files)
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
	got := paths(out)
	want := []string{"a.go", "c.go"}
	if !equal(got, want) {
		t.Errorf("Reorder = %v, want %v", got, want)
	}
}

func TestReorder_ExtraFilesAtEnd(t *testing.T) {
	// r lists b.go, PR has a.go, b.go, c.go → output is [b.go, a.go, c.go].
	r := &ReviewOrder{Entries: []ReviewOrderEntry{{Path: "b.go"}}}
	files := []ParsedFile{
		{NewPath: "a.go"},
		{NewPath: "b.go"},
		{NewPath: "c.go"},
	}
	out, dropped := r.Reorder(files)
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
	got := paths(out)
	want := []string{"b.go", "a.go", "c.go"}
	if !equal(got, want) {
		t.Errorf("Reorder = %v, want %v", got, want)
	}
}

func TestReorder_RenameMatchesOldPath(t *testing.T) {
	// File renamed old.go → new.go. Entry names old.go — should match.
	r := &ReviewOrder{Entries: []ReviewOrderEntry{{Path: "old.go"}}}
	files := []ParsedFile{
		{OldPath: "old.go", NewPath: "new.go", IsRename: true},
		{NewPath: "z.go"},
	}
	out, dropped := r.Reorder(files)
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
	got := paths(out)
	want := []string{"new.go", "z.go"} // the rename is moved to the front via OldPath match
	if !equal(got, want) {
		t.Errorf("Reorder = %v, want %v", got, want)
	}
}

func TestReorder_NilAndEmpty(t *testing.T) {
	files := []ParsedFile{{NewPath: "a.go"}, {NewPath: "b.go"}}

	var r *ReviewOrder
	out, dropped := r.Reorder(files)
	if dropped != 0 || !equal(paths(out), []string{"a.go", "b.go"}) {
		t.Errorf("nil Reorder = %v/%d", paths(out), dropped)
	}

	r = &ReviewOrder{}
	out, dropped = r.Reorder(files)
	if dropped != 0 || !equal(paths(out), []string{"a.go", "b.go"}) {
		t.Errorf("empty Reorder = %v/%d", paths(out), dropped)
	}
}

func TestNoteFor(t *testing.T) {
	r := &ReviewOrder{Entries: []ReviewOrderEntry{
		{Path: "a.go", Note: "first"},
		{Path: "old.go", Note: "renamed"},
	}}
	if got := r.NoteFor(ParsedFile{NewPath: "a.go"}); got != "first" {
		t.Errorf("NoteFor(a.go) = %q, want %q", got, "first")
	}
	if got := r.NoteFor(ParsedFile{OldPath: "old.go", NewPath: "new.go"}); got != "renamed" {
		t.Errorf("NoteFor(rename) = %q, want %q", got, "renamed")
	}
	if got := r.NoteFor(ParsedFile{NewPath: "unknown.go"}); got != "" {
		t.Errorf("NoteFor(unknown) = %q, want empty", got)
	}
	var nilR *ReviewOrder
	if got := nilR.NoteFor(ParsedFile{NewPath: "a.go"}); got != "" {
		t.Errorf("NoteFor on nil = %q, want empty", got)
	}
}

// writeOrderFile helpers.
func writeOrderFile(t *testing.T, dir, contents string) {
	t.Helper()
	sigilDir := filepath.Join(dir, ".sigil")
	if err := os.MkdirAll(sigilDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(sigilDir, "review-order.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func paths(files []ParsedFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.NewPath
		if f.IsDelete && f.OldPath != "" {
			out[i] = f.OldPath
		}
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
