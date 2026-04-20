---
id: sig-0t5m
status: open
deps: [sig-kota, sig-lmwr]
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, anchor]
---
# Context anchoring + re-anchor on drift

Build comment anchors (before[2], target, after[2]) from a diff hunk + line index. Re-anchor existing comments against a new diff; mark misses as orphaned.

## Design

New file `diff/anchor.go`. Depends on S1 (types) + S4 (parse).

Functions:
- BuildAnchor(hunk *ParsedHunk, lineIdx int, side Side) Context
    — extracts target + up to 2 before + up to 2 after from within the hunk; trims at hunk edges
- ReAnchor(comment *Comment, newDiff *ParsedDiff) (matched bool, newLineHint int, hunkHeader string)
    — searches newDiff for any hunk containing target with matching before[] immediately preceding and after[] immediately following; whitespace-sensitive exact byte match
- MarkOrphans(comments []*Comment, newDiff *ParsedDiff) (orphanedIDs []string)
    — runs ReAnchor per comment; mutates orphaned + line_hint + hunk_header in place; returns list of newly-orphaned IDs for banner

Match rule: exact byte equality including leading whitespace. Target required; before[] must immediately precede the target in the hunk line sequence; after[] must immediately follow. No fuzzy matching in v1.

## Acceptance Criteria

- BuildAnchor handles hunk-edge cases (fewer than 2 lines of context available)
- ReAnchor returns match + new line number when the hunk moved
- ReAnchor returns no-match when target deleted or edited
- MarkOrphans mutates in place and returns orphan list for banner
- Unit tests cover: hunk moved, file grew/shrank, target edited, target deleted, context before/after changed, anchor at hunk start, anchor at hunk end

