---
id: sig-7qra
status: closed
deps: []
links: []
created: 2026-04-21T00:09:06Z
type: task
priority: 2
assignee: James McKernan
parent: sig-8n5o
tags: [sigil, diff]
---
# Worktree path plumbing + stale-HEAD warning

Thread the worktree path from session resolve all the way into the TUI model so LSP has an absolute filesystem base. Also detect and warn when worktree HEAD differs from session.HeadSHA (LSP results will be stale against the diff snapshot).

## Design

Changes:
- diff/resolve.go: have Resolve return workspaceDir string alongside *Session and *ParsedDiff. Populate from PRCandidate.WorktreePath on the auto-detect path.
- For loadSessionByID (sigil diff -s <id>): after load, call listWorktrees(ctx, cwd) and match on session.Branch. If no match, return empty string.
- diff/tui/view.go: Run(session, pd, worktreePath string); RunWithResolve passes the workspaceDir through.
- diff/tui/model.go: Model gains worktreePath string; New(session, pd, worktreePath) signature.
- cli/diff.go: update call site.
- Stale-HEAD check: at model New or first update, run git -C worktreePath rev-parse HEAD; if result != session.HeadSHA, set m.statusMsg = 'LSP results may be stale: worktree HEAD differs from session snapshot'. Don't disable features.

LSP fields on the Model (lsp *lsp.Manager) are nil-safe so the TUI works fine with worktreePath='' — the go-to-def action in stream 4 will check and show a status error instead.

## Acceptance Criteria

- tui.Run / tui.New accept a worktreePath parameter.
- diff.Resolve returns workspaceDir for both autoDetect and loadSessionByID paths (empty string when worktree can't be found by branch).
- Stale-HEAD case produces a visible status-bar warning at TUI start; non-stale case is silent.
- go build ./... and go vet ./... pass; existing tests still green.
- Launching sigil diff in a normal worktree shows no warning and the TUI works as before.

