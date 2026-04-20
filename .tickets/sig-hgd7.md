---
id: sig-hgd7
status: open
deps: [sig-liqf, sig-6704, sig-jrmh, sig-cvr1, sig-eij7]
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, e2e]
---
# E2E wiring + integration tests

Wire the Kong default subcommand sigil diff (bare) to resolver → TUI. Build an integration harness with tempdir + git init + mock gh binary that drives the full flow end-to-end.

## Design

cli/diff_tui.go Run:
1. call diff.Resolve(ctx, ResolveOpts{SessionID: parent.Session, IncludeDraft: parent.Draft, CWD: cwd})
2. on ErrPickerNeeded → launch tui picker from S8 → re-resolve with chosen SessionID
3. on resolved session → tui.Run(session, diff)

Bare `sigil diff` defaults to TUI via DiffTUICmd `default:"withargs"`.

Integration test harness (diff/diff_integration_test.go):
- tempdir with `git init`, two commits on a feature branch
- fake origin set to github.com URL
- mock gh binary: shell script in $TMPDIR echoing canned JSON for pr list and canned unified diff for pr diff
- PATH override points to the mock

Test cases:
- Happy path: sigil diff (bare) → TUI opens → press c → type → Enter → q → verify comments.yaml
- --session override bypasses detection
- --draft includes draft PRs
- Zero PRs: clean error with `gh pr create` hint
- gh not installed: clean error
- Non-GH remote: clean error with hint
- Drift: advance head, rerun, verify new snapshot + orphan flag on affected comment

## Acceptance Criteria

- sigil diff with no args enters TUI on the detected PR
- sigil diff --session <id> skips detection
- sigil diff --draft includes drafts
- E2E integration test passes
- All error paths (missing gh, not authed, no PR, non-GH remote) produce actionable messages
- Drift scenario test: second run after head advance creates new snapshot and orphans the affected comment

