package diff

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// Worktree represents a single git worktree entry.
type Worktree struct {
	Path       string
	Branch     string
	HeadSHA    string
	IsDetached bool
	IsBare     bool
}

// ErrNotGitHub is returned when the origin remote is not a github.com host.
var ErrNotGitHub = errors.New("remote is not a GitHub host")

// ListWorktrees runs `git worktree list --porcelain` and returns all worktrees.
func ListWorktrees(ctx context.Context) ([]Worktree, error) {
	out, err := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parseWorktreePorcelain(string(out))
}

// parseWorktreePorcelain parses the output of `git worktree list --porcelain`.
func parseWorktreePorcelain(output string) ([]Worktree, error) {
	var worktrees []Worktree
	blocks := strings.Split(strings.TrimSpace(output), "\n\n")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		wt := Worktree{}
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.Path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "HEAD "):
				wt.HeadSHA = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				ref := strings.TrimPrefix(line, "branch ")
				// Strip refs/heads/ prefix if present.
				wt.Branch = strings.TrimPrefix(ref, "refs/heads/")
			case line == "detached":
				wt.IsDetached = true
			case line == "bare":
				wt.IsBare = true
			}
		}
		worktrees = append(worktrees, wt)
	}
	return worktrees, nil
}

// sshGitHubPattern matches git@github.com:owner/repo.git
var sshGitHubPattern = regexp.MustCompile(`^git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)

// OriginRemote returns the host, owner, and repo from the origin remote URL.
// Supports https://github.com/owner/repo, git@github.com:owner/repo.git,
// and ssh://git@github.com/owner/repo. Returns ErrNotGitHub for non-github.com.
func OriginRemote(ctx context.Context, cwd string) (host, owner, repo string, err error) {
	args := []string{"remote", "get-url", "origin"}
	if cwd != "" {
		args = append([]string{"-C", cwd}, args...)
	}
	out, execErr := exec.CommandContext(ctx, "git", args...).Output()
	if execErr != nil {
		return "", "", "", fmt.Errorf("git remote get-url origin: %w", execErr)
	}
	raw := strings.TrimSpace(string(out))
	return parseRemoteURL(raw)
}

func parseRemoteURL(raw string) (host, owner, repo string, err error) {
	// Try SSH SCP syntax: git@host:owner/repo.git
	if m := sshGitHubPattern.FindStringSubmatch(raw); m != nil {
		host = m[1]
		owner = m[2]
		repo = m[3]
		if host != "github.com" {
			return "", "", "", ErrNotGitHub
		}
		return host, owner, repo, nil
	}

	// Try https:// or ssh:// URL
	u, parseErr := url.Parse(raw)
	if parseErr != nil {
		return "", "", "", fmt.Errorf("parse remote URL %q: %w", raw, parseErr)
	}
	host = u.Hostname()
	if host != "github.com" {
		return "", "", "", ErrNotGitHub
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("unexpected path in remote URL %q", raw)
	}
	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")
	return host, owner, repo, nil
}

// CurrentBranch returns the current branch name, or empty string for detached HEAD.
func CurrentBranch(ctx context.Context, cwd string) (string, error) {
	args := []string{"rev-parse", "--abbrev-ref", "HEAD"}
	if cwd != "" {
		args = append([]string{"-C", cwd}, args...)
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "", nil
	}
	return branch, nil
}
