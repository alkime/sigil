---
id: sig-1d6u
status: closed
deps: [sig-1eaj]
links: []
created: 2026-03-25T13:56:23Z
type: task
priority: 2
assignee: James McKernan
parent: sig-hs1z
tags: [cli-subcommands]
---
# Implement resolve-comments and unresolve-comments subcommands

Context: LLMs need to mark comments resolved after addressing them, or reopen them.

Approach:
Create cli/resolve_comments.go with:
- ResolveCommentsCmd: File arg, IDs []string arg
- UnresolveCommentsCmd: same shape
- Shared setCommentStatus(file, ids, status, out) helper:
  - parser.Parse, normalize each ID, lookup in CommentByID
  - writer.UpdateComment(doc, id, existing.Comment, status)
  - Chain returned doc for sequential updates
  - Print confirmation to out

Key files: Create cli/resolve_comments.go
Reuse: writer.UpdateComment(), NormalizeID()

Verification:
- Resolve open comment -> status becomes "resolved"
- Unresolve -> status back to "open"
- Nonexistent ID returns clear error

