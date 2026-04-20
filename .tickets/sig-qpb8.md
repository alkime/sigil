---
id: sig-qpb8
status: closed
deps: []
links: []
created: 2026-04-20T19:27:34Z
type: task
priority: 2
assignee: James McKernan
tags: [backlog]
---
# Improve diff TUI line-selection highlight

Current selection indicator uses a '▸' gutter marker with indentation that looks off. Replace with a subtle full-line background highlight (no indentation shift) so the selected range is visually clear without disturbing the diff layout. Should feel similar to a terminal editor's visual-mode selection — low-contrast bg color spanning the entire row.

