---
id: sig-n6x6
status: open
deps: []
links: []
created: 2026-04-20T19:25:44Z
type: task
priority: 2
assignee: James McKernan
tags: [backlog]
---
# Add general PR-level comment support to sigil diff TUI

A comment not anchored to any file/line, analogous to GitHub top-level PR comments. Implementation: (1) use File="" sentinel in Comment struct — storage needs no changes, (2) add `C` keybinding in normal mode to open comment modal without line selection, (3) add a virtual "PR Comments" entry at top of file list pane navigable via Tab, reusing existing file-pane rendering.

