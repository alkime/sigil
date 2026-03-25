---
id: sig-hs1z
status: open
deps: []
links: []
created: 2026-03-25T13:54:35Z
type: epic
priority: 2
assignee: James McKernan
---
# Add CLI subcommands for LLM integration

Add Kong-based CLI subcommands to Sigil for programmatic LLM integration. Currently Sigil only has a TUI mode (sigil file.md). This epic adds subcommands: get-comments, resolve-comments, unresolve-comments, reply-comment, and install-skill.

New dependency: github.com/alecthomas/kong

Architecture: All subcommand logic in a new cli/ package. main.go becomes a thin Kong bootstrap. Default command (no subcommand) still launches the TUI.

## Work Streams

### cli-foundation (branch: cli-foundation)
T1 Kong bootstrap, CLI struct, TUI migration -> T2 Foundation tests

### cli-subcommands (branch: cli-subcommands)
T3 get-comments -> T4 resolve/unresolve-comments -> T5 reply-comment -> T6 install-skill -> T7 Subcommand tests
(blocked until cli-foundation completes T1)

