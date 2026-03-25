---
id: sig-1eaj
status: closed
deps: [sig-1q2f]
links: []
created: 2026-03-25T13:56:22Z
type: task
priority: 2
assignee: James McKernan
parent: sig-hs1z
tags: [cli-subcommands]
---
# Implement get-comments subcommand

Context: LLMs need to read all review comments from a Sigil file programmatically.

Approach:
Create cli/get_comments.go with:
- GetCommentsCmd struct: File arg, --open/--resolved flags (xor:"status")
- Run: parser.Parse, filter by status, print plain text blocks:

  === Comment 0001 [open] ===
  Lines: 4-7
  ---
  Comment text
  ---
  > source line 1
  > source line 2

- Line numbers: 1-based source lines (RefMarker.SourceLine + Offset + 1)
- Covered text from doc.RawLines at marker.SourceLine + comment.Offset for Span lines
- Write to ctx.Out for testability

Key files: Create cli/get_comments.go
Reuse: parser.Parse(), doc.RefMarkers, doc.CommentByID

Verification:
- sigil get-comments file.md prints all comments
- --open/--resolved filter correctly
- --open --resolved errors (mutually exclusive)

