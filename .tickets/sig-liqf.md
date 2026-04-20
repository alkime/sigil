---
id: sig-liqf
status: open
deps: [sig-lmwr, sig-kota]
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, tui]
---
# TUI shell: layout + diff render + keybindings

Bubble Tea v2 TUI with header / file list / diff viewport / keybar. Renders unified diff with chroma syntax highlighting. Keybindings for navigation. Multi-PR picker. No comment entry yet.

## Design

New package diff/tui/. Files:
- diff/tui/model.go — AppModel: {session, diff, files, currentFile, viewport, keymap, orphans[], mode}
- diff/tui/view.go — layout composition via lipgloss (header | body | keybar)
- diff/tui/render.go — unified diff rendering: chroma syntax highlighting for Add/Context lines; +/- coloring via lipgloss; line numbers gutter
- diff/tui/keymap.go — j/k, J/K (hunk jump), Tab (next file), n/N (next/prev comment), c (stub), r (stub), o (stub), q, ?
- diff/tui/picker.go — multi-PR picker for ErrPickerNeeded

Deps: charm.land/bubbletea/v2, charm.land/bubbles/v2 (viewport), charm.land/lipgloss/v2, alecthomas/chroma/v2 (promote to direct).

TUI entry: Run(session *Session, diff *ParsedDiff) error — called from cli/diff_tui.go (S11).

Non-commentable files (binary/rename/delete) are shown in list with "(no line context)" marker; c key is a no-op there with a brief hint.

## Acceptance Criteria

- TUI launches against a fixture session; renders header + file list + diff viewport
- j/k/J/K/Tab navigation works
- Multi-file: Tab cycles current file; sidebar highlights current
- chroma syntax highlighting applied to known extensions; plain text fallback
- Add vs Delete vs Context lines visually differentiated
- Non-commentable files listed with "(no line context)"; c is no-op + hint
- Multi-PR picker appears when invoked with candidate list
- TUI tests via Update(tea.Msg) pattern (mirror model/app_test.go); table-driven keybinding tests

