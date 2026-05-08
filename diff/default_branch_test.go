package diff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultBranch_FromGitSymbolicRef sets up a repo with origin/HEAD pointing
// at a non-"main" branch and confirms DefaultBranch reads that probe.
func TestDefaultBranch_FromGitSymbolicRef(t *testing.T) {
	dir := t.TempDir()

	// Build an upstream "remote" repo with a "production" branch.
	upstream := filepath.Join(dir, "upstream.git")
	runGitCmd(t, dir, "init", "--bare", "-b", "production", upstream)

	// Build a local repo, push to upstream, set origin/HEAD.
	local := filepath.Join(dir, "local")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, local, "init", "-b", "production")
	runGitCmd(t, local, "config", "user.email", "test@test.com")
	runGitCmd(t, local, "config", "user.name", "Test User")
	writeTestFile(t, local, "README.md", "# x\n")
	runGitCmd(t, local, "add", ".")
	runGitCmd(t, local, "commit", "-m", "init")
	runGitCmd(t, local, "remote", "add", "origin", upstream)
	runGitCmd(t, local, "push", "-u", "origin", "production")
	runGitCmd(t, local, "remote", "set-head", "origin", "production")

	got, err := DefaultBranch(context.Background(), local)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "production" {
		t.Errorf("DefaultBranch = %q, want %q", got, "production")
	}
}

// TestDefaultBranch_BothProbesFail asserts the error message names both probes
// so users can pick which to fix.
func TestDefaultBranch_BothProbesFail(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, "init", "-b", "main")
	runGitCmd(t, dir, "config", "user.email", "test@test.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")
	writeTestFile(t, dir, "f.txt", "x\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "init")
	// No remote, no origin/HEAD — git probe fails.
	// Stub gh that always exits non-zero — gh probe fails.
	stubDir := t.TempDir()
	stub := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(filepath.Join(stubDir, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := DefaultBranch(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error when both probes fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "symbolic-ref") {
		t.Errorf("error should mention git symbolic-ref probe, got: %v", err)
	}
	if !strings.Contains(msg, "gh repo view") {
		t.Errorf("error should mention gh probe, got: %v", err)
	}
}
