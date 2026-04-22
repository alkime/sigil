package cli

import (
	"fmt"
)

// Skill text is agent-facing; the reviewer-facing gd / go-to-definition
// feature is documented in README.md instead of here.
const skillContent = `# Sigil — Markdown Review Tool

Sigil is a terminal-based Markdown & PR Diff review tool. It lets humans and LLMs
collaborate through inline review comments embedded directly in Markdown files.

## Prerequisites

Before using Sigil commands, verify it is installed and accessible:

` + "```" + `bash
sigil --help
` + "```" + `

If the command is not found, install it with:

` + "```" + `bash
go install github.com/alkime/sigil@latest
` + "```" + `

## Workflow

1. A human opens a Markdown file in Sigil's TUI and adds line-level review
   comments (selection + annotation).
2. Comments are persisted as structured data within the file itself using HTML
   comment markers and a YAML backmatter block.
3. The file is handed to an LLM with instructions to address all open comments.
4. The LLM reads comments programmatically, makes changes, replies, and resolves.

## CLI Subcommands

### View comments

` + "```" + `bash
# Print all comments
sigil get-comments file.md

# Print only open (unresolved) comments
sigil get-comments --open file.md

# Print only resolved comments
sigil get-comments --resolved file.md
` + "```" + `

### Resolve / unresolve comments

` + "```" + `bash
# Resolve one or more comments by ID
sigil resolve-comments file.md 1 2 3

# Unresolve (reopen) comments
sigil unresolve-comments file.md 1
` + "```" + `

### Reply to a comment

` + "```" + `bash
sigil reply-comment file.md 1 "Fixed the typo, see updated text."
` + "```" + `

### Generate this skill file

` + "```" + `bash
sigil generate-skill > SKILL.md
` + "```" + `

## Typical LLM Workflow (Markdown mode)

1. Read open comments: ` + "`sigil get-comments --open file.md`" + `
2. Address each comment by editing the Markdown content.
3. Reply to confirm the change: ` + "`sigil reply-comment file.md <id> \"Done — updated wording.\"`" + `
4. Resolve when finished: ` + "`sigil resolve-comments file.md <id>`" + `

## PR Review Mode (sigil diff)

Sigil also supports reviewing GitHub PRs. Comments are stored locally and anchored
to diff context so they survive force-pushes and rebases.

### Invocation

` + "```" + `bash
cd my-repo
git checkout feature/x
sigil diff                  # open TUI (requires gh CLI)
` + "```" + `

### Diff subcommands

` + "```" + `bash
# Read all PR comments (default: current branch's open PR)
sigil diff get-comments
sigil diff get-comments --open
sigil diff get-comments --resolved
sigil diff get-comments --session <id>   # explicit session

# Reply to a comment
sigil diff reply-comment <comment-id> "addressed in latest commit"

# Resolve / unresolve
sigil diff resolve-comments <comment-id>
sigil diff unresolve-comments <comment-id>
` + "```" + `

### Typical agent workflow (PR mode)

1. ` + "`sigil diff get-comments --open`" + ` — read all open review comments
2. Address each comment by editing source files
3. ` + "`sigil diff reply-comment <id> \"Fixed\"`" + ` — acknowledge the change
4. ` + "`sigil diff resolve-comments <id>`" + ` — mark resolved

Session auto-detection uses git context: run any ` + "`sigil diff`" + ` subcommand from any
worktree and it finds the right PR automatically.

## Review Order (optional, recommended on PR creation)

When you open a PR as an agent, you have the clearest picture of which files a
human reviewer should read first and why. Capture that by writing a small YAML
file to the worktree root:

Path: ` + "`.sigil/review-order.yaml`" + `

` + "```" + `yaml
files:
  - path: api/auth.go
    note: new JWT middleware, start here
  - path: api/routes.go
    note: wires the middleware into routes
  - path: api/auth_test.go           # note is optional
` + "```" + `

Rules:
- Paths match ` + "`ParsedFile.NewPath`" + ` first, then ` + "`OldPath`" + ` (so renames are fine).
- Files you list that are not in the PR are silently ignored.
- Files in the PR that you don't list are appended at the end in diff order.
- ` + "`.sigil/`" + ` must be git-ignored — add it to ` + "`.gitignore`" + ` if it isn't already.

When present, ` + "`sigil diff`" + ` opens with your ordering and shows the note inline
next to the active file. ` + "`Shift+O`" + ` toggles between your order and the default
diff order. Keep notes short (one line).

## Comment Format

Each comment has:
- **ID**: Zero-padded 4-digit string (e.g., "0001")
- **Lines**: The source lines the comment covers
- **Comment**: The review text (may contain REPLY: sections)
- **Status**: "open" or "resolved"

Comments are stored as HTML comment blocks within the Markdown file:
- Inline ref markers: ` + "`<!-- @review-ref 0001 -->`" + ` placed above commented content
- YAML backmatter block at EOF inside ` + "`<!-- @review-backmatter ... -->`" + `
`

// Run prints the skill content to stdout.
func (c *GenerateSkillCmd) Run(ctx *CLIContext) error {
	fmt.Fprint(ctx.Out, skillContent)
	return nil
}
