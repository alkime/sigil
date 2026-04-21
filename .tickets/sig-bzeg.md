---
id: sig-bzeg
status: closed
deps: [sig-4iuw, sig-7qra, sig-pamc, sig-4rbg, sig-0dj6, sig-div5]
links: []
created: 2026-04-21T00:10:29Z
type: task
priority: 3
assignee: James McKernan
parent: sig-8n5o
tags: [sigil, diff, docs, qa]
---
# Manual QA + skill/README docs update

Run through the brief's Verification section end-to-end; update user-facing docs (README + cli/install_skill.go) so reviewers and downstream agents discover the new capability.

## Design

Manual QA checklist (from docs/architecture-briefs/sigil-diff-lsp-definition.md §Verification):
1. go build ./... && go vet ./... green.
2. go test ./lsp/... green.
3. Inside this repo, sigil diff against a test PR editing diff/tui/model.go:
   - On a line calling m.rebuildDiffView(), w onto the symbol, gd → jump to its definition (in-diff or out-of-diff depending on hunk coverage).
   - On a line calling diff.MarkOrphans(...), gd → ModeDefinition opens diff/anchor.go at MarkOrphans.
   - ctrl+o returns to the starting line.
   - gd in whitespace → 'no symbol under cursor', no mode change.
4. Stale-HEAD: check out a different commit in the worktree, sigil diff -s <id> → stale-snapshot warning visible.
5. rename gopls off PATH → 'gopls: executable not found' status, no crash.

Docs:
- README: add a short 'Go to definition' section (keys + supported languages).
- cli/install_skill.go: the skill text is aimed at LLMs consuming comments; only update if the reviewer-facing behavior change is relevant (likely a one-line note that reviewers can use gd; keep the agent interface section unchanged).

Not in scope: TypeScript / Python server configs (follow-up tickets).

## Acceptance Criteria

- All manual QA scenarios pass and are captured as notes on this ticket.
- README has a 'Go to definition' section with key bindings and current language support (Go only at GA).
- cli/install_skill.go reflects any reviewer-facing changes (or is explicitly marked as unchanged with rationale).
- No regressions in existing integration tests.

