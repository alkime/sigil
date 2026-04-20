---
id: sig-eij7
status: closed
deps: [sig-0t5m, sig-liqf]
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, tui, orphans]
---
# Orphan UX: banner + o cycle + stored-snapshot view

At TUI open, compute and surface orphaned comments: banner + o keybinding to cycle. Orphan view renders the original hunk (loaded from the stored snapshot) in a read-only pane.

## Design

Extend diff/tui/model.go + diff/tui/render.go.

On startup:
- diff.MarkOrphans(comments, currentDiff) returns orphan IDs → model.orphans
- If len > 0, render top-of-screen banner: "N orphaned comments — press o to review"

o enters orphan-mode:
- Load snapshot_ref's diff.patch from disk (diff/storage.go + diff/parse.go reuse)
- Render the original hunk the comment was written against, plus the comment body
- Keybar: [r] resolve  [u] unresolve  [o] next orphan  [Esc] exit
- No editing, no manual re-anchoring in v1

Write path for r/u: flip resolved + updated_at under flock.

## Acceptance Criteria

- Banner rendered when orphans exist; absent otherwise
- o cycles through orphans (wraps after last)
- Orphan view shows body + original hunk from snapshot_ref (read-only)
- r / u toggle resolved in comments.yaml
- Esc exits orphan mode back to main diff view
- Test fixture: comment anchored to a line since deleted; verify orphaned flag set and orphan view renders the stored context

