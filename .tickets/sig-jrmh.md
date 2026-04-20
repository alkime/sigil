---
id: sig-jrmh
status: closed
deps: [sig-kota, sig-6704]
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, cli]
---
# Kong sub-subcommands: get-comments / reply-comment / resolve

Expose sigil diff get-comments, reply-comment, resolve-comments, unresolve-comments under a Kong parent. Text output mirrors Markdown mode's get-comments exactly.

## Design

New files: cli/diff.go (parent), cli/diff_get_comments.go, cli/diff_reply_comment.go, cli/diff_resolve_comments.go.

Wire into cli/cli.go:12-21:
  Diff DiffCmd `cmd:"" name:"diff" help:"Local-first PR review TUI and agent interface."`

DiffCmd in cli/diff.go:
  type DiffCmd struct {
      Session       string                   `help:"Use specific session ID (skip auto-detect)."`
      Draft         bool                     `help:"Include draft PRs in auto-detect."`
      GetComments   DiffGetCommentsCmd       `cmd:"" name:"get-comments"`
      ReplyComment  DiffReplyCommentCmd      `cmd:"" name:"reply-comment"`
      Resolve       DiffResolveCmd           `cmd:"" name:"resolve-comments" aliases:"resolve-comment"`
      Unresolve     DiffUnresolveCmd         `cmd:"" name:"unresolve-comments" aliases:"unresolve-comment"`
      TUI           DiffTUICmd               `cmd:"" default:"withargs" hidden:""`
  }

Each sub-subcommand calls diff.Resolve(...) for the session then operates on comments.yaml.

Output format for get-comments (text, not JSON) — mirror cli/get_comments.go:
  === Comment <id> [open|resolved] ===
  File: <path>
  Hunk target: `<target>`
  ---
  <body>
  ---
  > <hunk_header>
  > <before_1>
  > <before_2>
  > <target>           (with diff marker prefix where applicable)
  > <after_1>
  > <after_2>

Reply = body = body + "\n\nREPLY: " + text; updated_at = now. Under flock.
Resolve/Unresolve = flip resolved bool; updated_at = now. Under flock.

All commands accept --author for override (default: git config user.name || $USER).

## Acceptance Criteria

- sigil diff get-comments prints blocks in the specified format
- --open / --resolved filters (xor) behave as in Markdown mode
- sigil diff reply-comment <id> "text" appends REPLY to body
- sigil diff resolve-comments <id>... sets resolved=true
- sigil diff unresolve-comments <id>... sets resolved=false
- All subcommands auto-detect session; --session <id> overrides
- --author flag overrides author on reply/resolve
- CLI tests with fixture session on disk (tempdir + seeded comments.yaml) verify stdout + on-disk state
- sigil --help groups `diff`; sigil diff --help shows all sub-subcommands

