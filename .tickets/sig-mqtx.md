---
id: sig-mqtx
status: closed
deps: [sig-1d6u]
links: []
created: 2026-03-25T13:56:25Z
type: task
priority: 2
assignee: James McKernan
parent: sig-hs1z
tags: [cli-subcommands]
---
# Implement reply-comment subcommand

Context: LLMs need to reply to review comments, creating threaded conversation.

Approach:
Create cli/reply_comment.go with:
- ReplyCommentCmd: File arg, ID arg, ReplyText arg
- Run: parser.Parse, NormalizeID, lookup in CommentByID
- Build: existing.Comment + "\n\nREPLY: " + cmd.ReplyText
- writer.UpdateComment(doc, id, newText, existing.Status) -- preserve status
- Print "Replied to 0001" to ctx.Out

Key files: Create cli/reply_comment.go
Reuse: writer.UpdateComment(), NormalizeID()

Verification:
- Reply appends text correctly
- Preserves existing status
- Nonexistent ID returns error

