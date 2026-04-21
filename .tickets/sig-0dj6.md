---
id: sig-0dj6
status: open
deps: [sig-7qra]
links: []
created: 2026-04-21T00:10:11Z
type: task
priority: 2
assignee: James McKernan
parent: sig-8n5o
tags: [sigil, diff, tui]
---
# ModeDefinition: full-file read-only viewer for out-of-diff targets

New TUI mode that renders a full source file (from the worktree) read-only, syntax-highlighted, scrolled to and highlighting the definition line. Supports recursive gd jumps into deeper definitions and return-to-diff via q/Esc.

## Design

New Mode constant (diff/tui/model.go): ModeDefinition.

State on Model (or a substruct): defFile string, defLine int, defSymbol string, defVP viewport.Model, defLines []string (raw), defMeta []lineInfo-like column mapping for recursive gd.

OpenDefinition(loc Location) handler (in diff/tui/definition.go from stream 4):
- Resolve URI to absolute path. If it's inside worktreePath and we have a matching ParsedFile with that NewLineNum in linesMeta → route to in-diff dispatch instead (stream 4 handles).
- Else: os.ReadFile(absPath); if fails, status error; abort.
- Tokenize with chroma via highlightLine (reuse render.go:462-490) per line.
- Build a viewport with the rendered content; compute line offsets so we can scroll to defLine-1.
- Render header: '<file>:<line>  definition of <symbol>' (reuse lipgloss border patterns from renderInspectModal at render.go:360-405).
- Highlight the target line with the existing focused-line background style.
- m.mode = ModeDefinition.

Key handling (updateDefinition):
- j/k/J/K → scroll defVP. h/l/w/b/e → column cursor on the focused line inside the viewer (reuse stream 3's cursor model).
- gd (or ctrl+]) → recurse: WordAt on current line → goToDefinition via the same pipeline; push jumpEntry so ctrl+o unwinds properly.
- q / Esc → pop jumpEntry (return to diff or previous definition).

External files (e.g. /pkg/mod) are opened the same way; header labels as '[external]' when absPath is not under worktreePath.

Don't allow commenting from ModeDefinition (comments require diff anchor context).

## Acceptance Criteria

- gd on an out-of-diff symbol opens ModeDefinition showing the target file syntax-highlighted.
- Target line is highlighted and visible in the viewport on open.
- j/k/J/K scroll the viewer; h/l/w/b/e move the column cursor.
- gd inside ModeDefinition recurses to another definition, jump history is maintained.
- q / Esc returns to the previous view (diff or earlier definition).
- External / stdlib files (read-only paths outside worktree) open with an '[external]' label.
- Failure to read the target file (e.g. permissions) shows a clean status error, no crash.

