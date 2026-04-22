---
id: sig-q1k3
status: open
deps: []
links: []
created: 2026-04-22T18:08:25Z
type: task
priority: 2
assignee: James McKernan
tags: [backlog]
---
# sigil diff: unify new-comment modal with inspect modal (fixes wrapping bug)

The renderCommentModal (new comment) and renderInspectModal (edit existing comment) currently diverge in both visuals and layout math. Two issues bundled:

1. Wrapping bug. In enterCommentMode (diff/tui/model.go ~line 913) the textarea width is set to min(m.width-8, 80). In renderCommentModal (diff/tui/render.go ~line 488) the modal interior is min(m.width-4, 80) - 6 (padding 1,2 + rounded border). So the textarea is ~6 columns wider than its container, causing the terminal to wrap continuation lines at col 0 without the left gutter - text looks broken and orphaned fragments appear below the gutter.

2. UX mismatch. The edit-existing modal looks much nicer: title with filename + status, meta line with author/timestamp, diff hunk viewport above the textarea, Tab to swap focus between hunk and textarea. The new-comment modal should match.

Fix (one go):
- Rewrite renderCommentModal to mirror renderInspectModal:
  - Title: 'New comment · <file.NewPath>' (file-level) or 'New PR-level comment' (fileIdx == -1)
  - Meta: 'by <resolveAuthor()> · now' (file-level), skip for PR-level
  - Hunk viewport (file-level only) - Tab swaps focus with textarea
  - Textarea with correct width
  - Footer: '[Ctrl+S] Submit  [Enter] Newline  [Esc] Cancel  [Tab] scroll diff' (Tab hint only file-level)
- Centralize width math in one helper (modalW := min(width-4, 80); innerW := modalW - 6) used by both modals so they cannot drift again.
- enterCommentMode (file-level) gets a hunk viewport similar to enterInspectMode, reusing hunkLines (already exists).
- PR-level new comment stays as the simple modal (no hunk to show).
- Add commentHunkVP viewport.Model and commentHunkFocus bool fields to Model, mirroring inspectHunkVP / inspectHunkFocus.
- Update updateComment to handle Tab (swap focus) when file-level.

Verification: open a PR comment modal, type a long paragraph - wrapping respects the gutter. File-level new comment shows the same layout as edit-existing. PR-level new comment still works (simpler modal).

