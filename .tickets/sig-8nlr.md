---
id: sig-8nlr
status: closed
deps: [sig-mqtx]
links: []
created: 2026-03-25T13:56:26Z
type: task
priority: 2
assignee: James McKernan
parent: sig-hs1z
tags: [cli-subcommands]
---
# Implement install-skill subcommand

Context: install-skill outputs a SKILL.md documenting all CLI commands for LLM consumption.

Approach:
Create cli/install_skill.go with:
- InstallSkillCmd struct (no args)
- Embedded skillContent string constant documenting:
  - What Sigil is and the review workflow
  - All CLI subcommands with usage examples
  - Typical LLM workflow: get-comments --open -> address -> reply -> resolve
- Run: fmt.Fprint(ctx.Out, skillContent)
- Usage: sigil install-skill > .claude/skills/sigil/SKILL.md

Key files: Create cli/install_skill.go

Verification:
- Prints valid Markdown to stdout
- Content is self-contained for LLM consumption

