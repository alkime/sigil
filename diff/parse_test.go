package diff

import (
	"testing"
)

var addDiff = []byte(`diff --git a/pkg/new.go b/pkg/new.go
new file mode 100644
--- /dev/null
+++ b/pkg/new.go
@@ -0,0 +1,3 @@
+package pkg
+
+func New() {}
`)

var deleteDiff = []byte(`diff --git a/pkg/old.go b/pkg/old.go
deleted file mode 100644
--- a/pkg/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package pkg
-
-func Old() {}
`)

var binaryDiff = []byte(`diff --git a/assets/logo.png b/assets/logo.png
index 17a971d..599f8dd 100644
Binary files a/assets/logo.png and b/assets/logo.png differ
`)

var renameDiff = []byte(`diff --git a/old.go b/new.go
similarity index 95%
rename from old.go
rename to new.go
--- a/old.go
+++ b/new.go
@@ -1,3 +1,3 @@
 package main

-func OldName() {}
+func NewName() {}
`)

var renameNoContentDiff = []byte(`diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
`)

var multiHunkDiff = []byte(`diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,4 +1,4 @@
 package main

-func A() {}
+func A2() {}

@@ -10,4 +10,4 @@

 func B() {
-	return
+	return nil
 }
`)

var multiFileDiff = []byte(`diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,2 +1,2 @@
-old a
+new a
 context
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,2 +1,2 @@
-old b
+new b
 context
`)

func TestParse_Add(t *testing.T) {
	pd, err := Parse(addDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pd.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(pd.Files))
	}
	f := pd.Files[0]
	if !f.IsAdd {
		t.Error("expected IsAdd=true")
	}
	if f.IsDelete || f.IsBinary || f.IsRename {
		t.Errorf("unexpected flags: delete=%v binary=%v rename=%v", f.IsDelete, f.IsBinary, f.IsRename)
	}
	if f.OldPath != "/dev/null" {
		t.Errorf("expected OldPath=/dev/null, got %q", f.OldPath)
	}
	if f.NewPath != "pkg/new.go" {
		t.Errorf("expected NewPath=pkg/new.go, got %q", f.NewPath)
	}
	if !f.IsCommentable() {
		t.Error("expected add file to be commentable")
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(f.Hunks))
	}
	h := f.Hunks[0]
	if h.OldStart != 0 || h.NewStart != 1 {
		t.Errorf("unexpected hunk starts: old=%d new=%d", h.OldStart, h.NewStart)
	}
	if len(h.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(h.Lines))
	}
	for i, l := range h.Lines {
		if l.Kind != LineAdd {
			t.Errorf("line %d: expected LineAdd, got %v", i, l.Kind)
		}
		if l.OldLineNum != 0 {
			t.Errorf("line %d: expected OldLineNum=0, got %d", i, l.OldLineNum)
		}
		if l.NewLineNum != int32(i+1) {
			t.Errorf("line %d: expected NewLineNum=%d, got %d", i, i+1, l.NewLineNum)
		}
	}
}

func TestParse_Delete(t *testing.T) {
	pd, err := Parse(deleteDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := pd.Files[0]
	if !f.IsDelete {
		t.Error("expected IsDelete=true")
	}
	if f.NewPath != "/dev/null" {
		t.Errorf("expected NewPath=/dev/null, got %q", f.NewPath)
	}
	if f.IsCommentable() {
		t.Error("expected delete file to not be commentable")
	}
	h := f.Hunks[0]
	for i, l := range h.Lines {
		if l.Kind != LineDelete {
			t.Errorf("line %d: expected LineDelete, got %v", i, l.Kind)
		}
		if l.OldLineNum != int32(i+1) {
			t.Errorf("line %d: expected OldLineNum=%d, got %d", i, i+1, l.OldLineNum)
		}
		if l.NewLineNum != 0 {
			t.Errorf("line %d: expected NewLineNum=0, got %d", i, l.NewLineNum)
		}
	}
}

func TestParse_Binary(t *testing.T) {
	pd, err := Parse(binaryDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := pd.Files[0]
	if !f.IsBinary {
		t.Error("expected IsBinary=true")
	}
	if f.IsCommentable() {
		t.Error("expected binary file to not be commentable")
	}
	if len(f.Hunks) != 0 {
		t.Errorf("expected 0 hunks for binary, got %d", len(f.Hunks))
	}
}

func TestParse_Rename(t *testing.T) {
	pd, err := Parse(renameDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := pd.Files[0]
	if !f.IsRename {
		t.Error("expected IsRename=true")
	}
	if f.OldPath != "old.go" {
		t.Errorf("expected OldPath=old.go, got %q", f.OldPath)
	}
	if f.NewPath != "new.go" {
		t.Errorf("expected NewPath=new.go, got %q", f.NewPath)
	}
	if !f.IsCommentable() {
		t.Error("expected rename-with-content to be commentable")
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(f.Hunks))
	}
}

func TestParse_RenameNoContent(t *testing.T) {
	pd, err := Parse(renameNoContentDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := pd.Files[0]
	if !f.IsRename {
		t.Error("expected IsRename=true")
	}
	if f.IsCommentable() {
		t.Error("expected rename-without-content to not be commentable")
	}
	if len(f.Hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(f.Hunks))
	}
}

func TestParse_MultiHunk(t *testing.T) {
	pd, err := Parse(multiHunkDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pd.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(pd.Files))
	}
	f := pd.Files[0]
	if len(f.Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(f.Hunks))
	}

	h1 := f.Hunks[0]
	if h1.OldStart != 1 || h1.NewStart != 1 {
		t.Errorf("hunk 0: unexpected starts old=%d new=%d", h1.OldStart, h1.NewStart)
	}

	h2 := f.Hunks[1]
	if h2.OldStart != 10 || h2.NewStart != 10 {
		t.Errorf("hunk 1: unexpected starts old=%d new=%d", h2.OldStart, h2.NewStart)
	}
}

func TestParse_MultiFile(t *testing.T) {
	pd, err := Parse(multiFileDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pd.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(pd.Files))
	}
	if pd.Files[0].OldPath != "a.go" || pd.Files[1].OldPath != "b.go" {
		t.Errorf("unexpected paths: %q %q", pd.Files[0].OldPath, pd.Files[1].OldPath)
	}
	for _, f := range pd.Files {
		if !f.IsCommentable() {
			t.Errorf("file %q should be commentable", f.NewPath)
		}
		h := f.Hunks[0]
		if len(h.Lines) != 3 {
			t.Fatalf("expected 3 lines in hunk of %q, got %d", f.OldPath, len(h.Lines))
		}
		kinds := []LineKind{LineDelete, LineAdd, LineContext}
		for i, l := range h.Lines {
			if l.Kind != kinds[i] {
				t.Errorf("%q line %d: expected kind %v, got %v", f.OldPath, i, kinds[i], l.Kind)
			}
		}
	}
}

func TestParse_LineNumbers(t *testing.T) {
	d := []byte(`diff --git a/f.go b/f.go
--- a/f.go
+++ b/f.go
@@ -5,5 +5,6 @@
 ctx1
 ctx2
-deleted
+added1
+added2
 ctx3
 ctx4
`)
	pd, err := Parse(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := pd.Files[0].Hunks[0]

	tests := []struct {
		kind       LineKind
		oldLineNum int32
		newLineNum int32
		text       string
	}{
		{LineContext, 5, 5, "ctx1"},
		{LineContext, 6, 6, "ctx2"},
		{LineDelete, 7, 0, "deleted"},
		{LineAdd, 0, 7, "added1"},
		{LineAdd, 0, 8, "added2"},
		{LineContext, 8, 9, "ctx3"},
		{LineContext, 9, 10, "ctx4"},
	}

	if len(h.Lines) != len(tests) {
		t.Fatalf("expected %d lines, got %d", len(tests), len(h.Lines))
	}
	for i, tt := range tests {
		l := h.Lines[i]
		if l.Kind != tt.kind {
			t.Errorf("line %d: kind: expected %v, got %v", i, tt.kind, l.Kind)
		}
		if l.OldLineNum != tt.oldLineNum {
			t.Errorf("line %d: OldLineNum: expected %d, got %d", i, tt.oldLineNum, l.OldLineNum)
		}
		if l.NewLineNum != tt.newLineNum {
			t.Errorf("line %d: NewLineNum: expected %d, got %d", i, tt.newLineNum, l.NewLineNum)
		}
		if l.Text != tt.text {
			t.Errorf("line %d: text: expected %q, got %q", i, tt.text, l.Text)
		}
	}
}
