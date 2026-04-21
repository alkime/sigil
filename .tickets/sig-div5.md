---
id: sig-div5
status: closed
deps: [sig-pamc, sig-4rbg, sig-0dj6]
links: []
created: 2026-04-21T00:10:17Z
type: task
priority: 3
assignee: James McKernan
parent: sig-8n5o
tags: [sigil, diff, tui, polish]
---
# Keymap + help modal + status bar polish

Expose the new bindings (h/l/w/b/e, gd, ctrl+], ctrl+o) in the help modal and the bottom key bar; wire them through the KeyMap struct; ensure status messages are consistent.

## Design

Files:
- diff/tui/keymap.go — new key.Binding fields on KeyMap: ColLeft, ColRight, WordNext, WordPrev, WordEnd, GoToDef, GoToDefAlt, JumpBack. Wire WithHelp labels.
- diff/tui/render.go: renderHelp() (line ~428) add entries for the new bindings grouped as 'Cursor' and 'Navigation'. renderKeyBar (line ~318) surface gd and ctrl+o (drop lower-priority entries if the bar overflows).
- Status-message consistency: centralize the LSP-related strings (e.g. 'no symbol under cursor', 'no LSP configured for .xyz', 'initializing gopls...', 'resolving definition...') as const strings in definition.go for easy review.

Mostly cosmetic / wiring. Should be a small diff.

## Acceptance Criteria

- Help modal (?) lists all new bindings with clear labels.
- Key bar shows at least gd and ctrl+o in its top-priority set.
- KeyMap fields drive the bindings (no hardcoded string literals in updateNormal beyond the existing pattern).
- Manual walkthrough: every new key binding is documented somewhere user-reachable.

