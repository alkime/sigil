---
id: sig-cvr1
status: closed
deps: [sig-jrmh, sig-0t5m, sig-liqf]
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, tui, comments]
---
# TUI comment creation (c keybinding)

Wire the c keybinding to open a textarea at the focused line, build a context anchor, and append a new Comment to comments.yaml under flock.

## Design

Extend diff/tui/model.go with a comment-entry submode.

Flow:
1. User presses c on a focused diff line (only if current file IsCommentable)
2. diff.BuildAnchor(currentHunk, focusedLineIdx, side) — side = right for Add/Context, left for Delete
3. Overlay a bubbles/textarea for body entry
4. Enter submits:
     new Comment{
       ID: uuid,
       File: currentFile.NewPath,
       HunkHeader: currentHunk.Header,
       LineHint: focusedLineDisplayNum,
       Side: side,
       Context: anchor,
       Body: textareaBuffer,
       Author: resolveAuthor(),  // git config user.name || $USER
       CreatedAt: now,
       UpdatedAt: now,
       Resolved: false,
       Orphaned: false,
       SnapshotRef: session.CurrentSnapshotRef(),
     }
5. Append to comments.yaml under flock; refresh in-memory comments list
6. Esc cancels without writing

Non-commentable files: c is no-op with a brief status-bar hint.

## Acceptance Criteria

- c opens textarea at focused line; multi-line input supported
- Enter persists a Comment with correct anchor (before/target/after), side, snapshot_ref
- Esc cancels cleanly without writing
- Author = git config user.name, fallback $USER
- Round-trip: sigil diff get-comments shows the newly-created comment
- c on non-commentable file is a no-op + hint
- TUI test drives c → type → Enter; asserts comments.yaml on disk matches expected shape

