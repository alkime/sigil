---
id: sig-sb3m
status: open
deps: [sig-hgd7]
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 2
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, docs]
---
# Docs + help polish + skill doc

README section for sigil diff, audit of all --help strings, update of sigil generate-skill output, usage walkthrough.

## Design

- README.md: add a "PR review" section with a short example:
    cd my-repo
    git checkout feature/x
    sigil diff                  # opens TUI for open PR on this branch
    sigil diff get-comments     # plain-text dump
    sigil diff reply-comment 0001 "fixed in next push"
    sigil diff resolve-comments 0001
- Audit --help strings across cli/diff*.go for clarity + consistency with Markdown mode
- Update cli/generate_skill.go (if it exists) to include diff subcommands so LLM users get the full surface
- docs/usage/diff.md: text-capture walkthrough of a typical review session (TUI keystrokes + resulting comments.yaml)

## Acceptance Criteria

- README has a PR review section with usage examples
- sigil --help and sigil diff --help are clear and consistent
- generate-skill output includes diff subcommands
- docs/usage/diff.md exists and walks through a typical session

