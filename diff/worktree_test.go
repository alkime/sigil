package diff

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const samplePorcelain = `worktree /repo
HEAD abc123def456abc123def456abc123def456abc123
branch refs/heads/main

worktree /repo/.worktrees/feature
HEAD 111222333444555666777888999aaabbbcccdddee
branch refs/heads/feature/my-thing

worktree /repo/.worktrees/detached
HEAD deadbeefdeadbeefdeadbeefdeadbeefdeadbeef
detached

worktree /repo/.worktrees/bare-wt
HEAD cafecafecafecafecafecafecafecafecafecafe
branch refs/heads/bare-branch
bare
`

func TestParseWorktreePorcelain(t *testing.T) {
	wts, err := parseWorktreePorcelain(samplePorcelain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 4 {
		t.Fatalf("expected 4 worktrees, got %d", len(wts))
	}

	// Main worktree
	if wts[0].Path != "/repo" {
		t.Errorf("wts[0].Path = %q, want /repo", wts[0].Path)
	}
	if wts[0].Branch != "main" {
		t.Errorf("wts[0].Branch = %q, want main", wts[0].Branch)
	}
	if wts[0].HeadSHA != "abc123def456abc123def456abc123def456abc123" {
		t.Errorf("wts[0].HeadSHA unexpected: %q", wts[0].HeadSHA)
	}
	if wts[0].IsDetached || wts[0].IsBare {
		t.Error("wts[0] should not be detached or bare")
	}

	// Feature worktree
	if wts[1].Branch != "feature/my-thing" {
		t.Errorf("wts[1].Branch = %q, want feature/my-thing", wts[1].Branch)
	}

	// Detached worktree
	if !wts[2].IsDetached {
		t.Error("wts[2] should be detached")
	}
	if wts[2].Branch != "" {
		t.Errorf("wts[2].Branch should be empty for detached HEAD, got %q", wts[2].Branch)
	}

	// Bare worktree
	if !wts[3].IsBare {
		t.Error("wts[3] should be bare")
	}
	if wts[3].Branch != "bare-branch" {
		t.Errorf("wts[3].Branch = %q, want bare-branch", wts[3].Branch)
	}
}

func TestParseWorktreePorcelainEmpty(t *testing.T) {
	wts, err := parseWorktreePorcelain("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(wts))
	}
}

var originRemoteTests = []struct {
	name    string
	url     string
	host    string
	owner   string
	repo    string
	wantErr error
}{
	{
		name:  "https",
		url:   "https://github.com/myorg/myrepo",
		host:  "github.com",
		owner: "myorg",
		repo:  "myrepo",
	},
	{
		name:  "https with .git suffix",
		url:   "https://github.com/myorg/myrepo.git",
		host:  "github.com",
		owner: "myorg",
		repo:  "myrepo",
	},
	{
		name:  "ssh scp syntax",
		url:   "git@github.com:myorg/myrepo.git",
		host:  "github.com",
		owner: "myorg",
		repo:  "myrepo",
	},
	{
		name:  "ssh scp without .git",
		url:   "git@github.com:myorg/myrepo",
		host:  "github.com",
		owner: "myorg",
		repo:  "myrepo",
	},
	{
		name:  "ssh:// URL",
		url:   "ssh://git@github.com/myorg/myrepo",
		host:  "github.com",
		owner: "myorg",
		repo:  "myrepo",
	},
	{
		name:    "gitlab returns ErrNotGitHub",
		url:     "https://gitlab.com/myorg/myrepo",
		wantErr: ErrNotGitHub,
	},
	{
		name:    "bitbucket returns ErrNotGitHub",
		url:     "git@bitbucket.org:myorg/myrepo.git",
		wantErr: ErrNotGitHub,
	},
	{
		name:    "self-hosted returns ErrNotGitHub",
		url:     "https://git.example.com/myorg/myrepo",
		wantErr: ErrNotGitHub,
	},
}

func TestParseRemoteURL(t *testing.T) {
	for _, tc := range originRemoteTests {
		t.Run(tc.name, func(t *testing.T) {
			host, owner, repo, err := parseRemoteURL(tc.url)
			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tc.host {
				t.Errorf("host = %q, want %q", host, tc.host)
			}
			if owner != tc.owner {
				t.Errorf("owner = %q, want %q", owner, tc.owner)
			}
			if repo != tc.repo {
				t.Errorf("repo = %q, want %q", repo, tc.repo)
			}
		})
	}
}

// TestListWorktreesViaMockedGit injects a fake git binary via PATH.
func TestListWorktreesViaMockedGit(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	err := os.WriteFile(script, []byte(fmt.Sprintf(`#!/bin/sh
cat <<'EOF'
%s
EOF
`, samplePorcelain)), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	wts, err := ListWorktrees(context.Background())
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(wts) != 4 {
		t.Fatalf("expected 4 worktrees, got %d", len(wts))
	}
}

// TestCurrentBranchViaMockedGit injects a fake git that returns a branch name.
func TestCurrentBranchViaMockedGit(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'my-feature'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	branch, err := CurrentBranch(context.Background(), "")
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "my-feature" {
		t.Errorf("branch = %q, want my-feature", branch)
	}
}

// TestCurrentBranchDetached checks that "HEAD" output maps to empty string.
func TestCurrentBranchDetached(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'HEAD'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	branch, err := CurrentBranch(context.Background(), "")
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "" {
		t.Errorf("detached HEAD should return empty string, got %q", branch)
	}
}
