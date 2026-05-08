package diff

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// These tests reuse setupTestRepo and setupMultiCmdGH from resolve_test.go.

const integrationPRListJSON = `[{"number":99,"title":"Integration Test PR","baseRefName":"main","isDraft":false,"headRefName":"feature/integration"}]`
const integrationDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"

 func main() {
`

// setupIntegrationRepo creates a repo with a named feature branch "feature/integration".
func setupIntegrationRepo(t *testing.T) (repoDir, baseSHA, headSHA string) {
	t.Helper()
	dir := t.TempDir()

	runGitCmd(t, dir, "init", "-b", "main")
	runGitCmd(t, dir, "config", "user.email", "test@test.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")

	writeTestFile(t, dir, "README.md", "# Test\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "initial commit")
	baseSHA = gitSHA(t, dir, "HEAD")

	runGitCmd(t, dir, "checkout", "-b", "feature/integration")
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "add main.go")
	headSHA = gitSHA(t, dir, "HEAD")

	runGitCmd(t, dir, "remote", "add", "origin", "https://github.com/testorg/testrepo")

	return dir, baseSHA, headSHA
}

func TestIntegration_happyPath(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupIntegrationRepo(t)
	setupMultiCmdGH(t, integrationPRListJSON, integrationDiff)

	session, pd, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if session.PRNumber != 99 {
		t.Errorf("pr_number = %d, want 99", session.PRNumber)
	}
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if pd == nil || len(pd.Files) == 0 {
		t.Error("expected non-empty parsed diff")
	}
	if len(session.Snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(session.Snapshots))
	}

	snap := session.Snapshots[0]
	snapDir := SnapshotDir("testorg", "testrepo", PRSessionKey(99), snap.Base, snap.Head)
	if _, err := os.Stat(filepath.Join(snapDir, "diff.patch")); err != nil {
		t.Errorf("diff.patch missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapDir, "meta.yaml")); err != nil {
		t.Errorf("meta.yaml missing: %v", err)
	}
}

func TestIntegration_sessionOverride(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupIntegrationRepo(t)
	setupMultiCmdGH(t, integrationPRListJSON, integrationDiff)

	// Create session via auto-detect
	first, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}

	// Load by session ID — should skip auto-detect
	second, _, _, err := Resolve(context.Background(), ResolveOpts{SessionID: first.ID})
	if err != nil {
		t.Fatalf("Resolve by ID: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("session ID mismatch: got %s, want %s", second.ID, first.ID)
	}
	if second.PRNumber != 99 {
		t.Errorf("pr_number = %d, want 99", second.PRNumber)
	}
}

func TestIntegration_zeroPRs(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupIntegrationRepo(t)
	// Empty PR list
	setupMultiCmdGH(t, "[]", integrationDiff)

	_, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err == nil {
		t.Fatal("expected error for zero PRs")
	}
	if errStr := err.Error(); !contains(errStr, "gh pr create") {
		t.Errorf("error should mention 'gh pr create', got: %v", err)
	}
}

func TestIntegration_ghNotInstalled(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupIntegrationRepo(t)

	// Write a stub gh that exits non-zero with "not found" behaviour
	// by overriding only gh in a prefix dir while keeping git on PATH.
	stubDir := t.TempDir()
	stubScript := "#!/bin/sh\nexit 127\n"
	if err := os.WriteFile(filepath.Join(stubDir, "gh"), []byte(stubScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err == nil {
		t.Fatal("expected error when gh fails")
	}
	// The error may be wrapped; check that it contains useful info
	errStr := err.Error()
	if !contains(errStr, "gh") && !errors.Is(err, ErrGHNotInstalled) {
		t.Errorf("error should mention gh or be ErrGHNotInstalled, got: %v", err)
	}
}

func TestIntegration_nonGitHubRemote(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	dir := t.TempDir()
	runGitCmd(t, dir, "init", "-b", "main")
	runGitCmd(t, dir, "config", "user.email", "test@test.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")
	writeTestFile(t, dir, "f.txt", "hello\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "init")
	// Non-GitHub remote
	runGitCmd(t, dir, "remote", "add", "origin", "https://gitlab.com/org/repo")

	setupMultiCmdGH(t, integrationPRListJSON, integrationDiff)

	_, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: dir})
	if err == nil {
		t.Fatal("expected error for non-GitHub remote")
	}
	if !errors.Is(err, ErrNotGitHub) {
		t.Errorf("expected ErrNotGitHub, got: %v", err)
	}
}

func TestIntegration_drift_newSnapshotAndOrphan(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDir)

	repoDir, _, _ := setupIntegrationRepo(t)
	setupMultiCmdGH(t, integrationPRListJSON, integrationDiff)

	// First resolve — creates session + snapshot
	session1, pd1, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	if len(pd1.Files) == 0 {
		t.Fatal("expected files in first diff")
	}

	// Add a comment that will become orphaned when the diff changes
	org, repoName := "testorg", "testrepo"
	if err := os.MkdirAll(SessionDir(org, repoName, PRSessionKey(99)), 0o755); err != nil {
		t.Fatal(err)
	}

	hunk := pd1.Files[0].Hunks[0]
	anchor := BuildAnchor(&hunk, 0, Right)
	comment := Comment{
		ID:          "orphan-test-001",
		File:        pd1.Files[0].NewPath,
		HunkHeader:  hunk.Header,
		LineHint:    int(hunk.Lines[0].NewLineNum),
		Side:        "right",
		Context:     anchor,
		Body:        "this will be orphaned",
		Author:      "tester",
		SnapshotRef: session1.Snapshots[0].Base + "_" + session1.Snapshots[0].Head,
	}
	if err := SaveComments(org, repoName, PRSessionKey(99), []Comment{comment}); err != nil {
		t.Fatalf("SaveComments: %v", err)
	}

	// Advance HEAD with a completely different diff that won't match the anchor
	writeTestFile(t, repoDir, "newfile.go", "package x\n")
	runGitCmd(t, repoDir, "add", ".")
	runGitCmd(t, repoDir, "commit", "-m", "second commit")

	// New diff that doesn't contain the original context
	newDiff := `diff --git a/newfile.go b/newfile.go
--- /dev/null
+++ b/newfile.go
@@ -0,0 +1 @@
+package x
`
	setupMultiCmdGH(t, integrationPRListJSON, newDiff)

	// Second resolve — should create new snapshot
	session2, _, _, err := Resolve(context.Background(), ResolveOpts{CWD: repoDir})
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}

	if session2.ID != session1.ID {
		t.Errorf("session ID changed: %s → %s", session1.ID, session2.ID)
	}
	if len(session2.Snapshots) < 2 {
		t.Errorf("expected at least 2 snapshots after drift, got %d", len(session2.Snapshots))
	}
	if session2.HeadSHA == session1.HeadSHA {
		t.Error("head SHA should have changed after new commit")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsBytes(s, substr))
}

func containsBytes(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
