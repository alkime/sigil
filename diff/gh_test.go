package diff

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeGH writes a shell script named "gh" into a temp dir and prepends it to PATH.
// The script cats a data file to stdout (or stderr) and exits with exitCode.
func fakeGH(t *testing.T, output string, exitCode int) {
	t.Helper()
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "gh_output.txt")
	if err := os.WriteFile(dataFile, []byte(output), 0o644); err != nil {
		t.Fatal(err)
	}
	var script string
	if exitCode == 0 {
		script = fmt.Sprintf("#!/bin/sh\ncat %q\n", dataFile)
	} else {
		script = fmt.Sprintf("#!/bin/sh\ncat %q >&2\nexit %d\n", dataFile, exitCode)
	}
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestGHPRListByHead_success(t *testing.T) {
	json := `[{"number":42,"title":"My PR","baseRefName":"main","isDraft":false,"headRefName":"feature"}]`
	fakeGH(t, json, 0)

	prs, err := GHPRListByHead(context.Background(), "org/repo", "feature", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 42 {
		t.Errorf("pr.Number = %d, want 42", prs[0].Number)
	}
	if prs[0].Title != "My PR" {
		t.Errorf("pr.Title = %q, want 'My PR'", prs[0].Title)
	}
}

func TestGHPRListByHead_emptyResult(t *testing.T) {
	fakeGH(t, "[]", 0)

	prs, err := GHPRListByHead(context.Background(), "org/repo", "no-such-branch", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected empty slice, got %d PRs", len(prs))
	}
}

func TestGHPRListByHead_unknownFields(t *testing.T) {
	// gh may add new fields in future — decoder must tolerate them
	json := `[{"number":7,"title":"T","baseRefName":"main","isDraft":false,"headRefName":"b","futureField":"x"}]`
	fakeGH(t, json, 0)

	prs, err := GHPRListByHead(context.Background(), "org/repo", "b", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 7 {
		t.Errorf("unexpected result: %+v", prs)
	}
}

func TestGHPRListByHead_notAuthed(t *testing.T) {
	fakeGH(t, "not logged in", 1)

	_, err := GHPRListByHead(context.Background(), "org/repo", "b", false)
	if !errors.Is(err, ErrGHNotAuthed) {
		t.Errorf("err = %v, want ErrGHNotAuthed", err)
	}
}

func TestGHPRListByHead_rateLimit(t *testing.T) {
	fakeGH(t, "API rate limit exceeded", 1)

	_, err := GHPRListByHead(context.Background(), "org/repo", "b", false)
	if !errors.Is(err, ErrGHRateLimit) {
		t.Errorf("err = %v, want ErrGHRateLimit", err)
	}
}

func TestGHPRDiff_success(t *testing.T) {
	diff := "diff --git a/foo.go b/foo.go\n+++ something\n"
	fakeGH(t, diff, 0)

	out, err := GHPRDiff(context.Background(), "org/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != diff {
		t.Errorf("diff mismatch: got %q", string(out))
	}
}

func TestGHPRDiff_notFound(t *testing.T) {
	fakeGH(t, "not found", 1)

	_, err := GHPRDiff(context.Background(), "org/repo", 999)
	if !errors.Is(err, ErrPRNotFound) {
		t.Errorf("err = %v, want ErrPRNotFound", err)
	}
}

func TestCheckGHAvailable_notInstalled(t *testing.T) {
	// Point PATH at empty dir so gh is not found.
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	err := CheckGHAvailable()
	if !errors.Is(err, ErrGHNotInstalled) {
		t.Errorf("err = %v, want ErrGHNotInstalled", err)
	}
}

func TestErrGHError(t *testing.T) {
	e := &ErrGH{Stderr: "something went wrong"}
	want := "gh error: something went wrong"
	if e.Error() != want {
		t.Errorf("ErrGH.Error() = %q, want %q", e.Error(), want)
	}
}
