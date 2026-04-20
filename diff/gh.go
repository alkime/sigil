package diff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GHPR holds the fields from `gh pr list --json`.
type GHPR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	BaseRefName string `json:"baseRefName"`
	IsDraft     bool   `json:"isDraft"`
	HeadRefName string `json:"headRefName"`
}

var (
	ErrGHNotInstalled = errors.New("gh CLI not found — install from https://cli.github.com/")
	ErrGHNotAuthed    = errors.New("gh not authenticated — run: gh auth login")
	ErrGHRateLimit    = errors.New("gh API rate limit exceeded")
	ErrPRNotFound     = errors.New("PR not found")
)

// ErrGH is the generic fallback when gh exits non-zero without a recognized pattern.
type ErrGH struct {
	Stderr string
}

func (e *ErrGH) Error() string { return fmt.Sprintf("gh error: %s", e.Stderr) }

// GHPRListByHead lists open PRs for a given repo whose head branch matches branch.
// Set includeDraft to true to include draft PRs.
func GHPRListByHead(ctx context.Context, repo, branch string, includeDraft bool) ([]GHPR, error) {
	args := []string{
		"pr", "list",
		"--repo", repo,
		"--state", "open",
		"--head", branch,
		"--json", "number,title,baseRefName,isDraft,headRefName",
	}
	if includeDraft {
		args = append(args, "--include-drafts")
	}
	out, stderr, err := runGH(ctx, args...)
	if err != nil {
		return nil, classifyGHError(err, stderr)
	}
	var prs []GHPR
	if err := json.NewDecoder(strings.NewReader(out)).Decode(&prs); err != nil {
		return nil, fmt.Errorf("gh pr list: decode JSON: %w", err)
	}
	return prs, nil
}

// GHPRDiff fetches the unified diff for a PR.
func GHPRDiff(ctx context.Context, repo string, prNumber int) ([]byte, error) {
	out, stderr, err := runGH(ctx, "pr", "diff", fmt.Sprintf("%d", prNumber), "--repo", repo)
	if err != nil {
		return nil, classifyGHError(err, stderr)
	}
	return []byte(out), nil
}

// CheckGHAvailable verifies that gh is installed and authenticated.
func CheckGHAvailable() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return ErrGHNotInstalled
	}
	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err != nil {
		s := string(out)
		if isNotAuthed(s) {
			return ErrGHNotAuthed
		}
		return &ErrGH{Stderr: s}
	}
	return nil
}

// runGH executes gh with the given arguments and returns stdout, stderr, and error.
func runGH(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func classifyGHError(err error, stderr string) error {
	if err != nil {
		if isNotInstalled(err) {
			return ErrGHNotInstalled
		}
	}
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "not found") || strings.Contains(s, "no pull requests"):
		return ErrPRNotFound
	case isRateLimit(s):
		return ErrGHRateLimit
	case isNotAuthed(s):
		return ErrGHNotAuthed
	default:
		return &ErrGH{Stderr: strings.TrimSpace(stderr)}
	}
}

func isNotInstalled(err error) bool {
	var execErr *exec.Error
	return errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound)
}

func isRateLimit(s string) bool {
	return strings.Contains(s, "rate limit") || strings.Contains(s, "api rate")
}

func isNotAuthed(s string) bool {
	return strings.Contains(s, "not logged in") ||
		strings.Contains(s, "authentication") ||
		strings.Contains(s, "not authenticated") ||
		strings.Contains(s, "auth login")
}
