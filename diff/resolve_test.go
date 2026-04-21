package diff

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// --- test helpers ---

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitSHA(t *testing.T, dir, ref string) string {
	t.Helper()
	sha, err := revParse(context.Background(), dir, ref)
	if err != nil {
		t.Fatalf("revParse %s: %v", ref, err)
	}
	return sha
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupTestRepo creates a real git repo with a main branch and a feature branch.
// Returns the repo dir, main (base) SHA, and feature HEAD SHA.
func setupTestRepo(t *testing.T) (repoDir, baseSHA, headSHA string) {
	t.Helper()
	dir := t.TempDir()

	runGitCmd(t, dir, "init", "-b", "main")
	runGitCmd(t, dir, "config", "user.email", "test@test.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")

	writeTestFile(t, dir, "README.md", "# Test\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "initial commit")
	baseSHA = gitSHA(t, dir, "HEAD")

	runGitCmd(t, dir, "checkout", "-b", "feature/my-pr")
	writeTestFile(t, dir, "foo.go", "package main\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "add foo.go")
	headSHA = gitSHA(t, dir, "HEAD")

	runGitCmd(t, dir, "remote", "add", "origin", "https://github.com/testorg/testrepo")

	return dir, baseSHA, headSHA
}

// setupMultiCmdGH writes a fake gh binary that handles pr list and pr diff.
func setupMultiCmdGH(t *testing.T, prListJSON, diffContent string) {
	t.Helper()
	dir := t.TempDir()

	listFile := filepath.Join(dir, "prlist.json")
	diffFile := filepath.Join(dir, "diff.patch")
	if err := os.WriteFile(listFile, []byte(prListJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diffFile, []byte(diffContent), 0o644); err != nil {
		t.Fatal(err)
	}

	script := "#!/bin/sh\n" +
		`if [ "$1" = "pr" ] && [ "$2" = "list" ]; then` + "\n" +
		`  cat ` + listFile + "\n" +
		`  exit 0` + "\n" +
		`fi` + "\n" +
		`if [ "$1" = "pr" ] && [ "$2" = "diff" ]; then` + "\n" +
		`  cat ` + diffFile + "\n" +
		`  exit 0` + "\n" +
		`fi` + "\n" +
		`if [ "$1" = "auth" ]; then` + "\n" +
		`  exit 0` + "\n" +
		`fi` + "\n" +
		`exit 1` + "\n"

	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const samplePRListJSON = `[{"number":42,"title":"My Feature","baseRefName":"main","isDraft":false,"headRefName":"feature/my-pr"}]`
const sampleDiff = "diff --git a/foo.go b/foo.go\n--- /dev/null\n+++ b/foo.go\n@@ -0,0 +1 @@\n+package main\n"

// --- tests ---

func TestResolve_newSession(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupTestRepo(t)
	setupMultiCmdGH(t, samplePRListJSON, sampleDiff)

	session, pd, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if session.PRNumber != 42 {
		t.Errorf("pr_number = %d, want 42", session.PRNumber)
	}
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if pd == nil || len(pd.Files) == 0 {
		t.Error("expected non-empty diff")
	}
	snap := session.Snapshots[0]
	snapDir := SnapshotDir("testorg", "testrepo", session.PRNumber, snap.Base, snap.Head)
	if _, err := os.Stat(filepath.Join(snapDir, "diff.patch")); err != nil {
		t.Errorf("diff.patch missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapDir, "meta.yaml")); err != nil {
		t.Errorf("meta.yaml missing: %v", err)
	}
	if len(session.Snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(session.Snapshots))
	}

	// sessions.yaml index should exist
	indexEntries, err := LoadIndex("testorg", "testrepo")
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if len(indexEntries) != 1 || indexEntries[0].PRNumber != 42 {
		t.Errorf("index entries: %+v", indexEntries)
	}
}

func TestResolve_existingSession_sameHead(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupTestRepo(t)
	setupMultiCmdGH(t, samplePRListJSON, sampleDiff)

	// First resolve creates the session.
	first, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	firstID := first.ID

	// Second resolve with same HEAD should not create a new snapshot.
	second, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if second.ID != firstID {
		t.Errorf("session ID changed: %s → %s", firstID, second.ID)
	}
	if len(second.Snapshots) != 1 {
		t.Errorf("expected still 1 snapshot after same-HEAD resolve, got %d", len(second.Snapshots))
	}
}

func TestResolve_existingSession_newHead(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupTestRepo(t)
	setupMultiCmdGH(t, samplePRListJSON, sampleDiff)

	// First resolve.
	_, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}

	// Advance HEAD on the feature branch.
	writeTestFile(t, repoDir, "bar.go", "package main\n")
	runGitCmd(t, repoDir, "add", ".")
	runGitCmd(t, repoDir, "commit", "-m", "add bar.go")

	// Second resolve with new HEAD should capture a new snapshot.
	second, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if len(second.Snapshots) != 2 {
		t.Errorf("expected 2 snapshots after HEAD advance, got %d", len(second.Snapshots))
	}
}

func TestResolve_bySessionID(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupTestRepo(t)
	setupMultiCmdGH(t, samplePRListJSON, sampleDiff)

	// Create session via auto-detect.
	created, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("auto-detect Resolve: %v", err)
	}

	// Reload by session ID.
	loaded, _, _, err := Resolve(context.Background(), ResolveOpts{SessionID: created.ID})
	if err != nil {
		t.Fatalf("Resolve by ID: %v", err)
	}
	if loaded.ID != created.ID {
		t.Errorf("loaded ID %s != created ID %s", loaded.ID, created.ID)
	}
}

func TestResolve_noPRs(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupTestRepo(t)
	setupMultiCmdGH(t, "[]", sampleDiff)

	_, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err == nil || !containsStr(err.Error(), "no open PRs") {
		t.Errorf("expected 'no open PRs' error, got: %v", err)
	}
}

func TestResolve_multiplePRs(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupTestRepo(t)
	multiJSON := `[{"number":1,"title":"PR One","baseRefName":"main","isDraft":false,"headRefName":"feature/my-pr"},` +
		`{"number":2,"title":"PR Two","baseRefName":"main","isDraft":false,"headRefName":"feature/my-pr"}]`
	setupMultiCmdGH(t, multiJSON, sampleDiff)

	_, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	var pickerErr *ErrPickerNeeded
	if !errors.As(err, &pickerErr) {
		t.Errorf("expected ErrPickerNeeded, got: %v", err)
	}
	if len(pickerErr.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(pickerErr.Candidates))
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStrIdx(s, sub))
}

func containsStrIdx(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
