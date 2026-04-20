---
id: sig-2omg
status: open
deps: []
links: []
created: 2026-04-20T16:45:37Z
type: epic
priority: 1
assignee: James McKernan
tags: [sigil, diff]
---
# sigil diff — local-first PR review TUI

Local-first, TUI-based code review annotation tool for GitHub PRs. Renders a PR diff in the terminal; users add inline comments anchored to diff context (not line numbers) so they survive force-pushes and rebases. Comments persist under $XDG_DATA_HOME/sigil/diffs/. Humans write via TUI; agents consume/reply/resolve via 'sigil diff get-comments|reply-comment|resolve-comments' subcommands that mirror Markdown mode's output format.

See docs/architecture-briefs/sigil-diff-mode.md for full design (review rounds 0001-0005 resolved).

Locked-in decisions:
- Reply = string concat (appends '\n\nREPLY: ...'), matches cli/reply_comment.go
- Canonical diff source = 'gh pr diff <N>'
- Package layout: new diff/ (session, snapshot, anchor, gh, storage) + diff/tui/ + cli/diff_*.go; parser/writer/model untouched
- Comment schema: new diff-mode type, does NOT extend model.ReviewComment

12 work streams decomposed into a DAG; see child tickets.

