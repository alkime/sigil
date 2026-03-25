---
id: sig-1q2f
status: closed
deps: []
links: []
created: 2026-03-25T13:55:47Z
type: task
priority: 2
assignee: James McKernan
parent: sig-hs1z
tags: [cli-foundation]
---
# Kong bootstrap, CLI struct, and TUI migration

Context: Sigil currently uses raw os.Args in main.go to launch the TUI. Replace with Kong-based CLI parsing to support subcommands while keeping default TUI behavior.

Approach:
1. go get github.com/alecthomas/kong
2. Create cli/cli.go with:
   - CLI struct with all subcommand fields (Kong struct tags)
   - CLIContext struct with Out io.Writer for testability
   - NormalizeID helper: "1" or "0001" -> "0001"
   - Stub command structs with Run(*CLIContext) error returning nil
3. Create cli/tui.go with:
   - TUICmd struct: File string arg, default:"withargs" hidden:""
   - TUICmd.Run: parser.Parse -> model.NewApp -> tea.NewProgram -> p.Run()
4. Rewrite main.go (~10 lines): kong.Parse, CLIContext{Out: os.Stdout}, ctx.Run

Key files:
- Create: cli/cli.go, cli/tui.go
- Modify: main.go, go.mod

Verification:
- go build ./... succeeds
- go vet ./... passes
- sigil file.md still launches TUI

