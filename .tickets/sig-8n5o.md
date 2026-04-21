---
id: sig-8n5o
status: open
deps: []
links: []
created: 2026-04-21T00:08:34Z
type: epic
priority: 2
assignee: James McKernan
tags: [sigil, diff, lsp]
---
# sigil diff — LSP go-to-definition (gopls first)

Add LSP-backed 'go to definition' to the sigil diff TUI. Reviewer presses gd on a symbol; if the definition is inside the current diff, the viewport jumps to it; otherwise a read-only ModeDefinition opens the target file. Gopls ships first; the lsp/ package is language-agnostic so TypeScript (typescript-language-server) and Python (pyright / pylsp) become one-line registry entries later.

## Design

See docs/architecture-briefs/sigil-diff-lsp-definition.md for the full design (JSON-RPC 2.0 framing rationale, worktree-path plumbing, column-cursor UX, in-diff vs out-of-diff rendering, jump history, edge cases).

Locked-in decisions:
- Protocol: JSON-RPC 2.0 over stdio (mandated by LSP spec; OpenAPI/REST not applicable — LSP is bidirectional + stdio-framed). Roll our own minimal codec; no go.lsp.dev/protocol dep.
- Symbol selection UX: column cursor + vi motions (h/l/w/b/e) + gd. Line-picker alternative rejected.
- Out-of-diff target rendering: full-file read-only viewer (not excerpt) using existing highlightLine + viewport patterns.
- Worktree path: derived at TUI runtime (not persisted in Session); degrades gracefully when not found.
- No new module deps: lsp/ is self-contained in the sigil module.

Work streams decomposed into child tickets below. Streams 1/2/3 are independent and parallelizable; 4 blocks on all three; 5 blocks on 2; 6 and 7 tail.

## Acceptance Criteria

- Reviewer can press gd on a Go symbol (add-side or context) in a diff and land on its definition.
- In-diff definition: viewport scrolls to and highlights the definition line.
- Out-of-diff definition: ModeDefinition opens the target file, syntax-highlighted, scrolled to the definition line.
- ctrl+o returns to the previous location.
- gopls initialization is non-blocking (status bar shows 'initializing gopls…').
- Missing gopls binary shows a clean status error (no crash).
- Stale worktree HEAD (diff snapshot doesn't match disk) shows a warning at TUI start.
- go build ./... and go vet ./... pass; go test ./lsp/... passes with a fake LSP server.

