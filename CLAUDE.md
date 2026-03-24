# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Sigil is a terminal-based Markdown viewer with inline review commenting. Users read Glamour-rendered Markdown in a TUI, select line ranges, and attach comments — all persisted as structured data within the Markdown file itself using HTML comments.

Primary workflow: human reviews LLM-generated Markdown → adds line-level comments via TUI → hands file back to LLM with "address all open review comments" → LLM resolves and updates.

## Build & Run

```bash
go build ./...                          # Build all packages
go run . <file.md>                      # Run with a markdown file
go test ./...                           # Run all tests
go test -run TestName ./parser/...      # Run a single parser test
go vet ./...                            # Static analysis
```

## Architecture

- `main.go` — CLI entry, arg parsing, `tea.NewProgram`
- `parser/` — Parses Markdown files into `Document` model: extracts `<!-- @review-ref NNNN -->` markers, YAML backmatter block, builds content-line-to-source-line mappings
- `model/` — Bubbletea v2 models: `AppModel` (top-level state machine), viewport with gutter markers, inspect modal, status bar
- `writer/` — Writes ref markers + backmatter back to file (stub, M2)
- `model/types.go` — Core domain types shared across packages: `Document`, `ReviewComment`, `RefMarker`

## Comment Format

Inline ref markers (`<!-- @review-ref 0001 -->`) placed above commented content, plus a YAML backmatter block at EOF inside `<!-- @review-backmatter ... -->`. YAML keys are quoted strings (`"0001":`). Fields: offset, span, comment, status.

## Key Conventions

- Bubbletea v2 API: `Init() tea.Cmd`, `Update(tea.Msg) (tea.Model, tea.Cmd)`, `View() tea.View`
- Key matching: `tea.KeyPressMsg` + `msg.String()` or `key.Matches()`
- Viewport from `charm.land/bubbles/v2/viewport` — uses `LeftGutterFunc` for gutter markers, `SetHighlights` for n/N navigation
- All review data lives in the file itself (no sidecar files, no database)
- IDs are zero-padded 4-digit strings ("0001", "0002", ...)

## Dependencies

- `charm.land/bubbletea/v2` — TUI framework
- `charm.land/glamour/v2` — Markdown rendering
- `charm.land/lipgloss/v2` — Styling
- `charm.land/bubbles/v2` — Reusable TUI components (viewport, key bindings)
- `gopkg.in/yaml.v3` — YAML parsing for backmatter
