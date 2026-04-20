---
id: sig-2z7r
status: closed
deps: []
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, gh]
---
# GH integration: gh pr list + gh pr diff wrappers

Subprocess wrappers around the gh CLI for PR discovery (gh pr list --head <branch>) and diff retrieval (gh pr diff <N>), with a distinct error taxonomy for install/auth/rate-limit/not-found.

## Design

New file `diff/gh.go`. Shell out to `gh` binary via os/exec — no external library dependency.

Functions:
- GHPRListByHead(ctx, repo, branch, includeDraft) ([]GHPR, error)
    → gh pr list --repo <repo> --state open --head <branch> [--include-drafts] --json number,title,baseRefName,isDraft,headRefName
- GHPRDiff(ctx, repo, prNumber) ([]byte, error)
    → gh pr diff <N> --repo <repo>
- CheckGHAvailable() error → verifies binary on PATH and `gh auth status` clean

Error taxonomy (sentinel errors in diff/gh.go):
- ErrGHNotInstalled — hint install docs URL
- ErrGHNotAuthed — hint `gh auth login`
- ErrGHRateLimit
- ErrPRNotFound
- Generic ErrGH(stderr) fallback

JSON parsing of `gh pr list` tolerates unknown fields (forward-compat).

## Acceptance Criteria

- diff/gh.go implements GHPRListByHead, GHPRDiff, CheckGHAvailable
- Sentinel errors distinguish install / auth / rate-limit / not-found
- Unit tests use a fake gh binary injected via PATH (shell script in $TMPDIR) to simulate success, not-installed, not-authed, empty-result, rate-limit
- JSON decoder ignores unknown fields
- CheckGHAvailable returns copy-pastable hint strings suitable for CLI error output

