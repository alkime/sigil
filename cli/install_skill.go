package cli

import (
	"fmt"
)

const skillContent = `# Sigil — Markdown Review Tool

Sigil is a terminal-based Markdown review tool. It lets humans and LLMs
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

## Typical LLM Workflow

1. Read open comments: ` + "`sigil get-comments --open file.md`" + `
2. Address each comment by editing the Markdown content.
3. Reply to confirm the change: ` + "`sigil reply-comment file.md <id> \"Done — updated wording.\"`" + `
4. Resolve when finished: ` + "`sigil resolve-comments file.md <id>`" + `

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
