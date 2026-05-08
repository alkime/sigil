package diff

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultBranch returns the name of the upstream default branch (e.g. "main",
// "master", "production"). It tries two probes:
//
//  1. `git symbolic-ref --short refs/remotes/origin/HEAD` — set by git clone,
//     fast and offline.
//  2. `gh repo view --json defaultBranchRef --jq .defaultBranchRef.name` —
//     online fallback when origin/HEAD is missing/stale.
//
// On both failures returns an error mentioning each probe so the user can
// fix the underlying issue (e.g. `git remote set-head origin --auto`).
func DefaultBranch(ctx context.Context, cwd string) (string, error) {
	branch, gitErr := defaultBranchFromGit(ctx, cwd)
	if gitErr == nil && branch != "" {
		return branch, nil
	}
	branch, ghErr := defaultBranchFromGH(ctx, cwd)
	if ghErr == nil && branch != "" {
		return branch, nil
	}
	return "", fmt.Errorf(
		"couldn't detect default branch — pass --base <ref>:\n"+
			"  git symbolic-ref refs/remotes/origin/HEAD: %v\n"+
			"  gh repo view --json defaultBranchRef: %v",
		gitErr, ghErr,
	)
}

func defaultBranchFromGit(ctx context.Context, cwd string) (string, error) {
	args := []string{"symbolic-ref", "--short", "refs/remotes/origin/HEAD"}
	if cwd != "" {
		args = append([]string{"-C", cwd}, args...)
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(out))
	// Strip "origin/" prefix to leave the bare branch name.
	return strings.TrimPrefix(ref, "origin/"), nil
}

func defaultBranchFromGH(ctx context.Context, cwd string) (string, error) {
	args := []string{"repo", "view", "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name"}
	cmd := exec.CommandContext(ctx, "gh", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
