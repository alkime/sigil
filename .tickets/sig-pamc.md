---
id: sig-pamc
status: closed
deps: []
links: []
created: 2026-04-21T00:09:19Z
type: task
priority: 2
assignee: James McKernan
parent: sig-8n5o
tags: [sigil, diff, tui]
---
# Column cursor + vi motions (h/l/w/b/e) on focused line

Add a column-level cursor on the currently focused diff line with vi-style motions. Foundational for symbol selection; keeps sigil's vi-idiomatic feel.

## Design

Model changes (diff/tui/model.go):
- Add focusedCol int.
- Reset focusedCol on line change (j/k/J/K/Tab/shift+tab); snap to nearest word start (or 0).

Key bindings (diff/tui/keymap.go, KeyMap struct + DefaultKeyMap):
- h / l → focusedCol -= 1 / += 1, clamped to [0, len(lineText)-1].
- w / b / e → word motions. Word class: unicode.IsLetter || unicode.IsDigit || '_'. Ignore hunk-header and comment-marker lines (no motion).

Word-boundary scanner: small helper in diff/tui/words.go (pure function, easy to unit test). Expose WordAt(text string, col int) (start, end int, ok bool) for stream 4 to reuse.

Rendering (diff/tui/render.go):
- Re-render the focused line with inverse-video (or bg override) at focusedCol. Layer on top of the existing focused-line background styling. Don't touch non-focused lines.
- Skip cursor rendering when focused line is a hunk-header or comment-marker.

Line text access: use the raw ParsedLine.Text (pattern at model.go:663). Column indexes rune positions into that text (not into the rendered +/-/space-prefixed output).

## Acceptance Criteria

- h/l/w/b/e move the cursor visibly on the focused line; cursor renders as an inverse-video rune.
- Cursor hides on hunk-header and comment-marker lines.
- Line movement (j/k/J/K/Tab) resets focusedCol predictably.
- WordAt() unit tests cover: cursor in word, cursor in whitespace, cursor on punctuation, cursor at start/end of line, unicode identifier chars.
- No regressions in existing diff navigation tests.

