package diff

import (
	"context"
	"strings"
	"testing"
)

func TestLocalSessionKey_Sanitization(t *testing.T) {
	cases := []struct {
		branch, base, want string
	}{
		{"feature/x", "main", "local-feature_x-vs-main"},
		{"a/b/c", "release/1.0", "local-a_b_c-vs-release_1.0"},
		{"plain", "production", "local-plain-vs-production"},
	}
	for _, tc := range cases {
		got := string(LocalSessionKey(tc.branch, tc.base))
		if got != tc.want {
			t.Errorf("LocalSessionKey(%q,%q) = %q, want %q", tc.branch, tc.base, got, tc.want)
		}
		if !SessionKey(got).IsLocal() {
			t.Errorf("LocalSessionKey(%q,%q) not detected as local", tc.branch, tc.base)
		}
	}
}

func TestPRSessionKey_NotLocal(t *testing.T) {
	if PRSessionKey(42).IsLocal() {
		t.Error("PRSessionKey should not be classified as local")
	}
}

func TestKeyForSession_PR(t *testing.T) {
	s := &Session{Kind: SessionKindPR, PRNumber: 7}
	if got := KeyForSession(s); got != PRSessionKey(7) {
		t.Errorf("KeyForSession PR: got %q, want %q", got, PRSessionKey(7))
	}
}

func TestKeyForSession_Local(t *testing.T) {
	s := &Session{Kind: SessionKindLocal, Branch: "feat", BaseBranch: "main"}
	want := LocalSessionKey("feat", "main")
	if got := KeyForSession(s); got != want {
		t.Errorf("KeyForSession local: got %q, want %q", got, want)
	}
}

func TestResolve_local_createsSession(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	repoDir, baseSHA, headSHA := setupTestRepo(t)

	session, pd, wt, err := Resolve(context.Background(), ResolveOpts{
		CWD:     repoDir,
		Local:   true,
		BaseRef: "main", // explicit; auto-detect requires `git remote set-head`
	})
	if err != nil {
		t.Fatalf("Resolve --local: %v", err)
	}

	if session.Kind != SessionKindLocal {
		t.Errorf("Kind = %q, want local", session.Kind)
	}
	if session.PRNumber != 0 {
		t.Errorf("PRNumber = %d, want 0", session.PRNumber)
	}
	if session.Branch != "feature/my-pr" {
		t.Errorf("Branch = %q, want feature/my-pr", session.Branch)
	}
	if session.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want main", session.BaseBranch)
	}
	if session.HeadSHA != headSHA {
		t.Errorf("HeadSHA = %q, want %q", session.HeadSHA, headSHA)
	}
	if session.BaseSHA != baseSHA {
		t.Errorf("BaseSHA = %q, want %q", session.BaseSHA, baseSHA)
	}
	if wt != repoDir {
		t.Errorf("worktree path = %q, want %q", wt, repoDir)
	}
	if pd == nil || len(pd.Files) == 0 {
		t.Error("expected non-empty parsed diff")
	}
	if len(session.Snapshots) != 1 {
		t.Errorf("snapshots = %d, want 1", len(session.Snapshots))
	}

	// Second resolve should reuse the same session and skip re-snapshotting.
	again, _, _, err := Resolve(context.Background(), ResolveOpts{
		CWD: repoDir, Local: true, BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if again.ID != session.ID {
		t.Errorf("session ID changed across resolves: %q vs %q", again.ID, session.ID)
	}
	if len(again.Snapshots) != 1 {
		t.Errorf("snapshots after re-resolve = %d, want 1 (no churn)", len(again.Snapshots))
	}
}

func TestResolve_local_baseImpliesLocal(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	repoDir, _, _ := setupTestRepo(t)

	// Local=false, BaseRef set → resolveLocal still runs.
	session, _, _, err := Resolve(context.Background(), ResolveOpts{
		CWD:     repoDir,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("Resolve --base only: %v", err)
	}
	if session.Kind != SessionKindLocal {
		t.Errorf("Kind = %q, want local", session.Kind)
	}
}

func TestResolve_local_sameBranchAsBase(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	repoDir, _, _ := setupTestRepo(t)
	runGitCmd(t, repoDir, "checkout", "main")

	_, _, _, err := Resolve(context.Background(), ResolveOpts{
		CWD: repoDir, Local: true, BaseRef: "main",
	})
	if err == nil {
		t.Fatal("expected error when current branch equals base ref")
	}
	if !strings.Contains(err.Error(), "same as base") {
		t.Errorf("error = %q, want mention of 'same as base'", err.Error())
	}
}

func TestResolve_local_capturesNewSnapshotOnHEADChange(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	repoDir, _, _ := setupTestRepo(t)

	first, _, _, err := Resolve(context.Background(), ResolveOpts{
		CWD: repoDir, Local: true, BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}

	// Add another commit on the feature branch.
	writeTestFile(t, repoDir, "bar.go", "package main\n")
	runGitCmd(t, repoDir, "add", ".")
	runGitCmd(t, repoDir, "commit", "-m", "add bar.go")

	second, _, _, err := Resolve(context.Background(), ResolveOpts{
		CWD: repoDir, Local: true, BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("session ID changed: %q vs %q", second.ID, first.ID)
	}
	if len(second.Snapshots) != 2 {
		t.Errorf("snapshots = %d, want 2 after HEAD change", len(second.Snapshots))
	}
}
