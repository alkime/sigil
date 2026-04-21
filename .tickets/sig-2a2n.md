---
id: sig-2a2n
status: open
deps: []
links: []
created: 2026-04-21T15:09:08Z
type: task
priority: 2
assignee: James McKernan
tags: [backlog, sigil, diff, tui]
---
# sigil diff: in-file grep search

Add '/' to open a small search input within the current file's diff view. Regex toggle on by default (Ctrl+r to flip to literal). Search scans all hunks in the current file, jumps to the first match, and highlights matches via the existing viewport highlight machinery.

While search is active, 'n'/'N' cycle matches (context-sensitive: shadow the existing comment-next/prev bindings and revert to comment navigation when the search is cleared). 'Esc' dismisses and clears highlights.

Motivation: needed for practical testing of the LSP go-to-def feature on real diffs — currently hard to locate a specific symbol in a large multi-hunk file without search.

## Design

- New ModeSearch state with a text input + regex toggle
- Reuse the textinput pattern from ModeComment
- Iterate m.files[m.fileIdx].Hunks[*].Lines[*].Text; collect rendered line indices of matches
- Reuse viewport.SetHighlights (or equivalent) for match overlay
- While ModeSearch is active (or a searchActive flag is set post-dismiss), bind n/N to cycle matches; otherwise they keep their comment-next/prev meaning
- Esc from ModeSearch clears m.searchMatches and restores n/N semantics

## Acceptance Criteria

- '/' opens a search input at the bottom of the screen
- Default regex-on; a visible toggle (Ctrl+r or similar) switches to literal match
- Pressing Enter jumps to the first match in the current file, highlights all matches
- 'n' cycles to next match, 'N' to previous (context-sensitive while search is active)
- Esc (or clearing the query) dismisses search and restores default n/N behavior
- No regression in existing comment-navigation behavior when no search is active

