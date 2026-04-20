package diff

import (
	"reflect"
	"testing"
)

// hunkWith builds a ParsedHunk from a slice of (kind, text) pairs starting at given line numbers.
func hunkWith(oldStart, newStart int32, lines []ParsedLine) ParsedHunk {
	return ParsedHunk{
		OldStart: oldStart,
		OldLines: int32(len(lines)),
		NewStart: newStart,
		NewLines: int32(len(lines)),
		Header:   "@@ test @@",
		Lines:    lines,
	}
}

func ctx(b []string, target string, a []string) CommentContext {
	return CommentContext{Before: b, Target: target, After: a}
}

// --- BuildAnchor tests ---

func TestBuildAnchor_MiddleRightSide(t *testing.T) {
	hunk := hunkWith(1, 1, []ParsedLine{
		{Kind: LineContext, OldLineNum: 1, NewLineNum: 1, Text: "line A"},
		{Kind: LineContext, OldLineNum: 2, NewLineNum: 2, Text: "line B"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 3, Text: "target"},
		{Kind: LineContext, OldLineNum: 3, NewLineNum: 4, Text: "line C"},
		{Kind: LineContext, OldLineNum: 4, NewLineNum: 5, Text: "line D"},
	})
	got := BuildAnchor(&hunk, 2, Right)
	want := ctx([]string{"line A", "line B"}, "target", []string{"line C", "line D"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBuildAnchor_SkipsDeleteOnRightSide(t *testing.T) {
	// Delete line should be invisible from Right side
	hunk := hunkWith(1, 1, []ParsedLine{
		{Kind: LineContext, OldLineNum: 1, NewLineNum: 1, Text: "before"},
		{Kind: LineDelete, OldLineNum: 2, NewLineNum: 0, Text: "deleted"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 2, Text: "target"},
		{Kind: LineContext, OldLineNum: 3, NewLineNum: 3, Text: "after"},
	})
	got := BuildAnchor(&hunk, 2, Right)
	// Visible right: [before, target, after] — delete is skipped
	want := ctx([]string{"before"}, "target", []string{"after"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBuildAnchor_SkipsAddOnLeftSide(t *testing.T) {
	hunk := hunkWith(1, 1, []ParsedLine{
		{Kind: LineContext, OldLineNum: 1, NewLineNum: 1, Text: "before"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 2, Text: "added"},
		{Kind: LineDelete, OldLineNum: 2, NewLineNum: 0, Text: "target"},
		{Kind: LineContext, OldLineNum: 3, NewLineNum: 3, Text: "after"},
	})
	got := BuildAnchor(&hunk, 2, Left)
	// Visible left: [before, target, after] — add is skipped
	want := ctx([]string{"before"}, "target", []string{"after"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBuildAnchor_AtHunkStart(t *testing.T) {
	hunk := hunkWith(1, 1, []ParsedLine{
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 1, Text: "target"},
		{Kind: LineContext, OldLineNum: 1, NewLineNum: 2, Text: "after1"},
		{Kind: LineContext, OldLineNum: 2, NewLineNum: 3, Text: "after2"},
	})
	got := BuildAnchor(&hunk, 0, Right)
	want := ctx([]string{}, "target", []string{"after1", "after2"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBuildAnchor_AtHunkEnd(t *testing.T) {
	hunk := hunkWith(1, 1, []ParsedLine{
		{Kind: LineContext, OldLineNum: 1, NewLineNum: 1, Text: "before1"},
		{Kind: LineContext, OldLineNum: 2, NewLineNum: 2, Text: "before2"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 3, Text: "target"},
	})
	got := BuildAnchor(&hunk, 2, Right)
	want := ctx([]string{"before1", "before2"}, "target", []string{})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBuildAnchor_OnlyOneLine(t *testing.T) {
	hunk := hunkWith(1, 1, []ParsedLine{
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 1, Text: "alone"},
	})
	got := BuildAnchor(&hunk, 0, Right)
	want := ctx([]string{}, "alone", []string{})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// --- ReAnchor / MarkOrphans tests ---

func diffWithHunk(file string, oldStart, newStart int32, lines []ParsedLine) *ParsedDiff {
	return &ParsedDiff{Files: []ParsedFile{{
		OldPath: file,
		NewPath: file,
		Hunks:   []ParsedHunk{hunkWith(oldStart, newStart, lines)},
	}}}
}

func commentRight(file, target string, before, after []string, lineHint int) *Comment {
	return &Comment{
		ID:       "c1",
		File:     file,
		Side:     "right",
		LineHint: lineHint,
		Context:  CommentContext{Before: before, Target: target, After: after},
	}
}

func commentLeft(file, target string, before, after []string, lineHint int) *Comment {
	return &Comment{
		ID:       "c1",
		File:     file,
		Side:     "left",
		LineHint: lineHint,
		Context:  CommentContext{Before: before, Target: target, After: after},
	}
}

func TestReAnchor_HunkMoved(t *testing.T) {
	// Comment was at line 5, hunk moved to line 20
	c := commentRight("main.go", "target line", []string{"before1", "before2"}, []string{"after1"}, 5)
	newDiff := diffWithHunk("main.go", 18, 18, []ParsedLine{
		{Kind: LineContext, OldLineNum: 18, NewLineNum: 18, Text: "before1"},
		{Kind: LineContext, OldLineNum: 19, NewLineNum: 19, Text: "before2"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 20, Text: "target line"},
		{Kind: LineContext, OldLineNum: 20, NewLineNum: 21, Text: "after1"},
	})
	matched, hint, header := ReAnchor(c, newDiff)
	if !matched {
		t.Fatal("expected match")
	}
	if hint != 20 {
		t.Errorf("expected hint=20, got %d", hint)
	}
	if header == "" {
		t.Error("expected non-empty hunk header")
	}
}

func TestReAnchor_FileGrew(t *testing.T) {
	// New lines inserted before target, so target moved down
	c := commentRight("f.go", "the target", []string{"ctx1"}, []string{"ctx2"}, 3)
	newDiff := diffWithHunk("f.go", 1, 1, []ParsedLine{
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 1, Text: "new inserted line"},
		{Kind: LineContext, OldLineNum: 1, NewLineNum: 2, Text: "ctx1"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 3, Text: "the target"},
		{Kind: LineContext, OldLineNum: 2, NewLineNum: 4, Text: "ctx2"},
	})
	matched, hint, _ := ReAnchor(c, newDiff)
	if !matched {
		t.Fatal("expected match after file grew")
	}
	if hint != 3 {
		t.Errorf("expected hint=3, got %d", hint)
	}
}

func TestReAnchor_FileShrunk(t *testing.T) {
	// Lines removed before target, so target moved up
	c := commentRight("f.go", "the target", []string{"ctx1"}, []string{}, 10)
	newDiff := diffWithHunk("f.go", 3, 3, []ParsedLine{
		{Kind: LineContext, OldLineNum: 3, NewLineNum: 3, Text: "ctx1"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 4, Text: "the target"},
	})
	matched, hint, _ := ReAnchor(c, newDiff)
	if !matched {
		t.Fatal("expected match after file shrunk")
	}
	if hint != 4 {
		t.Errorf("expected hint=4, got %d", hint)
	}
}

func TestReAnchor_TargetEdited(t *testing.T) {
	c := commentRight("f.go", "original line", []string{"ctx"}, []string{}, 5)
	newDiff := diffWithHunk("f.go", 4, 4, []ParsedLine{
		{Kind: LineContext, OldLineNum: 4, NewLineNum: 4, Text: "ctx"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 5, Text: "edited line"},
	})
	matched, _, _ := ReAnchor(c, newDiff)
	if matched {
		t.Error("expected no match when target was edited")
	}
}

func TestReAnchor_TargetDeleted(t *testing.T) {
	c := commentRight("f.go", "will be deleted", []string{}, []string{}, 5)
	newDiff := diffWithHunk("f.go", 5, 5, []ParsedLine{
		{Kind: LineContext, OldLineNum: 5, NewLineNum: 5, Text: "other line"},
	})
	matched, _, _ := ReAnchor(c, newDiff)
	if matched {
		t.Error("expected no match when target was deleted")
	}
}

func TestReAnchor_ContextBeforeChanged(t *testing.T) {
	c := commentRight("f.go", "target", []string{"original before"}, []string{}, 5)
	newDiff := diffWithHunk("f.go", 4, 4, []ParsedLine{
		{Kind: LineContext, OldLineNum: 4, NewLineNum: 4, Text: "changed before"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 5, Text: "target"},
	})
	matched, _, _ := ReAnchor(c, newDiff)
	if matched {
		t.Error("expected no match when before context changed")
	}
}

func TestReAnchor_ContextAfterChanged(t *testing.T) {
	c := commentRight("f.go", "target", []string{}, []string{"original after"}, 5)
	newDiff := diffWithHunk("f.go", 4, 4, []ParsedLine{
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 4, Text: "target"},
		{Kind: LineContext, OldLineNum: 4, NewLineNum: 5, Text: "changed after"},
	})
	matched, _, _ := ReAnchor(c, newDiff)
	if matched {
		t.Error("expected no match when after context changed")
	}
}

func TestMarkOrphans_MutatesAndReturnsNewlyOrphaned(t *testing.T) {
	c1 := &Comment{
		ID: "c1", File: "f.go", Side: "right", LineHint: 5,
		Context: CommentContext{Target: "target1"},
	}
	c2 := &Comment{
		ID: "c2", File: "f.go", Side: "right", LineHint: 10,
		Context: CommentContext{Target: "target2"},
	}
	c3 := &Comment{
		ID: "c3", File: "f.go", Side: "right", Orphaned: true,
		Context: CommentContext{Target: "already orphaned"},
	}

	newDiff := diffWithHunk("f.go", 1, 1, []ParsedLine{
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 1, Text: "target1"},
		{Kind: LineContext, OldLineNum: 1, NewLineNum: 2, Text: "other"},
		// target2 absent — c2 becomes orphaned
	})

	orphaned := MarkOrphans([]*Comment{c1, c2, c3}, newDiff)

	if c1.Orphaned {
		t.Error("c1 should not be orphaned")
	}
	if c1.LineHint != 1 {
		t.Errorf("c1 LineHint: expected 1, got %d", c1.LineHint)
	}

	if !c2.Orphaned {
		t.Error("c2 should be orphaned")
	}

	// c3 was already orphaned — should NOT appear in returned list
	if len(orphaned) != 1 || orphaned[0] != "c2" {
		t.Errorf("expected orphaned=[c2], got %v", orphaned)
	}
}

func TestReAnchor_AnchorAtHunkStart(t *testing.T) {
	// Comment with no before context (was at hunk start)
	c := commentRight("f.go", "target", []string{}, []string{"after1"}, 1)
	newDiff := diffWithHunk("f.go", 5, 5, []ParsedLine{
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 5, Text: "target"},
		{Kind: LineContext, OldLineNum: 5, NewLineNum: 6, Text: "after1"},
		{Kind: LineContext, OldLineNum: 6, NewLineNum: 7, Text: "after2"},
	})
	matched, hint, _ := ReAnchor(c, newDiff)
	if !matched {
		t.Fatal("expected match at hunk start")
	}
	if hint != 5 {
		t.Errorf("expected hint=5, got %d", hint)
	}
}

func TestReAnchor_AnchorAtHunkEnd(t *testing.T) {
	// Comment with no after context (was at hunk end)
	c := commentRight("f.go", "target", []string{"before1"}, []string{}, 9)
	newDiff := diffWithHunk("f.go", 8, 8, []ParsedLine{
		{Kind: LineContext, OldLineNum: 8, NewLineNum: 8, Text: "before1"},
		{Kind: LineAdd, OldLineNum: 0, NewLineNum: 9, Text: "target"},
	})
	matched, hint, _ := ReAnchor(c, newDiff)
	if !matched {
		t.Fatal("expected match at hunk end")
	}
	if hint != 9 {
		t.Errorf("expected hint=9, got %d", hint)
	}
}
