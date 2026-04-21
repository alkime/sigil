# LSP "Go to Definition" for sigil diff

## Context

The sigil diff TUI currently lets reviewers navigate hunks and leave line-level comments, but it has no way to follow a symbol to its definition — reviewers must alt-tab to an editor for that. We want reviewers to select a symbol on the current diff line and jump to its definition:

- If the definition is inside the current diff → scroll the diff viewport to that line.
- If the definition is outside the diff → show a read-only excerpt of the target file.

Definitions are resolved via LSP. Gopls ships first; TypeScript (`typescript-language-server`) and Python (`pyright` / `pylsp`) are planned follow-ups, so the design must be language-pluggable from day one.

## Current state (what's in place vs. missing)

**In place** (file paths + lines):
- Diff TUI model & rendering: `diff/tui/model.go`, `diff/tui/render.go`.
- Line metadata that maps viewport line → `(hunkIdx, lineInHunk, LineKind, old/new line number)`: `render.go:43-52`, populated in `render.go:69-137`.
- Raw line text via `m.files[fileIdx].Hunks[hunkIdx].Lines[lineInHunk].Text` (pattern used at `model.go:663`).
- Modal patterns: `ModeComment`, `ModeInspect`, `ModeHelp`, `ModeOrphan` in `model.go:30-39`, `renderInspectModal()` in `render.go:360-405`.
- Solid `os/exec` + `context` subprocess plumbing with stdout/stderr capture: `diff/gh.go:84-92` (`runGH`), `diff/worktree.go:30-40`.
- Worktree enumeration: `diff.ListWorktrees()` / `listWorktrees()` in `diff/worktree.go:25-40`. We can re-derive the worktree from `session.Branch` at TUI runtime.
- Chroma syntax-highlighted line rendering: `render.go:462-490` (`highlightLine`) — reusable for out-of-diff excerpts.
- Keymap: `diff/tui/keymap.go`.

**Missing:**
- Any LSP / JSON-RPC infrastructure (verified against `go.mod`).
- Any column cursor or word-under-cursor concept — cursor is purely `focusedLine int`.
- Worktree path is not plumbed into the TUI (`Run(session, pd)` at `view.go:14` takes only the session + diff; `Session` has `Branch` but no `WorktreePath`).
- Any jump-back / location-history stack.
- Any full-file / out-of-diff file viewing; only hunks are rendered.

## Recommended approach

Build this in layers so the LSP core stays language-agnostic and each layer is separately testable.

### 1. New package: `lsp/`

Minimal, purpose-built JSON-RPC 2.0 + LSP client — no external LSP library. Our scope is narrow (`initialize`, `initialized`, `textDocument/didOpen`, `textDocument/definition`, `shutdown`, `exit`); a library like `go.lsp.dev/protocol` is overkill and heavy. We can graduate later if we add hover/references/rename.

**Why JSON-RPC 2.0 specifically (and not OpenAPI/Swagger):** LSP is a spec — published by Microsoft and implemented by every language server we care about (gopls, typescript-language-server, pyright, pylsp, rust-analyzer) — and that spec *mandates* JSON-RPC 2.0 over stdio, pipe, or socket. It's not a design choice on our side; it's the contract the server expects. Two structural reasons the LSP authors chose JSON-RPC over OpenAPI-style REST:
1. **Bidirectional and long-lived.** The server pushes notifications to the client unsolicited (`textDocument/publishDiagnostics`, `window/showMessage`, progress updates). OpenAPI describes request/response HTTP endpoints; it has no model for server→client push.
2. **Stdio transport, not HTTP.** LSP clients spawn the server as a child process and talk over its stdin/stdout (Content-Length framed). There's no HTTP server to describe with an OpenAPI document.

The practical upside: the JSON-RPC 2.0 framing we need is tiny (a Content-Length header, a JSON body, id correlation for request/response). `lsp/jsonrpc.go` is ~100 lines. We're not paying a library tax for it.

```
lsp/
  jsonrpc.go      // Content-Length framed JSON-RPC 2.0 codec over io.ReadWriteCloser
  types.go        // Narrow LSP structs: InitializeParams, Position, Location, TextDocumentIdentifier, ...
  client.go       // Client: spawn, handshake, request/response correlation, didOpen cache, Close
  server.go       // ServerConfig: binary, args, root marker files (e.g. go.mod), init options
  registry.go     // FileExtensionToServer: ".go" → gopls config
  registry_test.go
  client_test.go  // Fake LSP server via shell script (mirrors diff/gh_test.go fakeGH pattern)
```

Key design points:
- One `Client` per (language, project root) pair; cached in a `Manager` keyed on root.
- JSON-RPC I/O runs on a goroutine reading stdout; responses delivered via `map[id]chan response` with mutex.
- Requests take a `context.Context` so the TUI can cancel.
- `Definition(ctx, absFile, line, col) ([]Location, error)` is the only public RPC in v1. `Location` carries `URI`, `Range.Start{Line,Character}` in 0-indexed LSP coordinates.
- On first use of a file, `Client` auto-`didOpen`s it with the current disk contents (required by gopls; it won't resolve definitions in files it doesn't know about).
- `Manager.Close()` sends `shutdown` + `exit`, waits briefly, then kills the process.

### 2. Plumb the worktree path into the TUI

The session doesn't persist it, and we don't want to couple LSP to the diff snapshot's stored state.

- Change `tui.Run(session, pd)` → `tui.Run(session, pd, worktreePath)` in `diff/tui/view.go:14`.
- In `RunWithResolve` (`view.go:23-49`), determine the worktree path:
  - Auto-detect path: already computed — pipe it through from `resolveCandidate` (currently discarded after resolve). Small refactor: have `diff.Resolve` also return `workspaceDir string`.
  - `-s SessionID` path: after `loadSessionByID`, call `listWorktrees(ctx, cwd)` and match by `session.Branch`. If none matches, pass `""` — LSP features degrade gracefully (status bar: "LSP disabled: worktree not found").
- `Model` gains `worktreePath string` and an `lsp *lsp.Manager` (nil when disabled).

### 3. Column cursor on the focused line

Cursor becomes `(focusedLine, focusedCol int)`. `focusedCol` is a rune index into the trimmed line text (no +/-/space prefix).

New bindings in `diff/tui/keymap.go` (added to `KeyMap` struct and `DefaultKeyMap()`):
- `h` / `l` → column −1 / +1, clamped to line length.
- `w` / `b` / `e` → word motions via a simple word-boundary scanner (`unicode.IsLetter || IsDigit || '_'`).
- On line change (j/k/J/K/Tab), reset `focusedCol = 0` or snap to nearest word start.

Visual cursor: re-render the focused line at render time with an inverse-video rune at `focusedCol`. Add to `render.go` alongside the existing `highlightLine` pipeline — the selection styling already covers focused-line background; we add one-rune emphasis on top.

### 4. Go-to-definition action

New key binding: `gd` (two-key chord; sigil's existing keys are single-key, so we store a "last key" shard in `Model` — small addition to `updateNormal`). Alternative single-key: `ctrl+]` (vi jump-to-tag). Plan ships both.

Flow in a new `model.go` handler `goToDefinition`:

1. Identify the symbol:
   - Get current `lineInfo` via `m.linesMeta[m.focusedLine]`.
   - Reject hunk-header and comment-marker lines.
   - Get raw line text: `hunk.Lines[meta.lineInHunk].Text`.
   - Extract word at `focusedCol` using the same word-boundary scanner used for `w`/`b`. If cursor is in whitespace/punctuation → status bar: "no symbol under cursor".

2. Resolve the file on disk:
   - `side := lineKindToSide(meta.lineKind)` — `Add` and `Context` use the new file / `NewLineNum`; `Delete` uses the old file / `OldLineNum`.
   - For MVP we only support `Add`/`Context` (Delete-side requires checking out the base SHA, which is out of scope). Status bar: "go-to-def on deleted lines not supported" when rejected.
   - `absFile := filepath.Join(worktreePath, file.NewPath)`.
   - `lspLine := int(line.NewLineNum - 1)`, `lspCol := column offset within the raw file line` (byte or UTF-16 code-unit offset — start with rune-count, document the nuance; gopls tolerates rune offsets for ASCII and most Go code).

3. Pick LSP server:
   - `cfg, ok := lsp.ForExtension(filepath.Ext(absFile))`. If none registered, status bar: "no LSP configured for .xyz".
   - `client, err := manager.Get(ctx, cfg, projectRoot)` — projectRoot found by walking up from `absFile` for `cfg.RootMarker` (e.g. `go.mod`), falling back to `worktreePath`.

4. Send request asynchronously via `tea.Cmd`:
   - Return a `tea.Cmd` that calls `client.Definition(ctx, absFile, lspLine, lspCol)` and emits a new message `defResultMsg{locations, err}`.
   - Keep a `lspReqCancel context.CancelFunc` on the Model so repeated `gd` presses cancel in-flight requests.
   - Show status "resolving definition…" immediately.

5. Handle `defResultMsg`:
   - **No results / error** → status bar error.
   - **One result** → decide in-diff vs out-of-diff (below).
   - **Multiple results** → reuse `pickerModel` pattern (`diff/tui/picker.go`) to list candidates.

### 5. Showing the target

**In-diff target** — the definition's `(file, line)` matches a `ParsedFile` + a `ParsedLine.NewLineNum` currently rendered:
- Look up the rendered line index via a reverse index over `linesMeta` (build once per `rebuildDiffView`).
- Switch `fileIdx` if needed; call `rebuildDiffView()`; set `focusedLine` + `focusedCol`; call `ensureLineVisible()`.
- Push a `jumpEntry{fileIdx, focusedLine, focusedCol}` onto the history stack.

**Out-of-diff target** — new `ModeDefinition`:
- Read the full target file from `worktreePath`.
- Render the full file (not just an excerpt) with chroma via `highlightLine`, inside a `viewport.Model` similar to `inspectHunkVP`.
- Header shows `file:line  definition of <symbol>`.
- Scroll to target line; highlight it (background style).
- Keys: `j/k/J/K` scroll; `q` / `Esc` return to diff; `gd` on a symbol in this view recurses (pushes another history entry).

A full-file viewer is only marginally more work than an excerpt viewer and is the right default — real codebases often have the definition far from any hunk, and an excerpt is frustrating. The mode is read-only; no commenting from here.

### 6. Jump history

`Model.jumpHistory []jumpEntry` (bounded, e.g. 32 entries). Entries record mode + location.
- `ctrl+o` → pop and restore.
- `ctrl+i` → redo (optional for MVP; can defer).

### 7. Keymap, help, status bar

- Add bindings to `KeyMap` struct and `DefaultKeyMap()` in `diff/tui/keymap.go`.
- Extend help modal list at `render.go:428-440`.
- Add `renderKeyBar` entries in `render.go:318-326` for `gd` and `ctrl+o`.

### 8. gopls server config (initial)

```go
// lsp/registry.go
var defaultServers = map[string]ServerConfig{
    ".go": {
        Language:   "go",
        Binary:     "gopls",
        Args:       nil,
        RootMarkers: []string{"go.mod", "go.work"},
    },
}
```

MVP: hardcoded map. Later: user override via a `sigil.yaml` config or env vars (out of scope for this plan).

### 9. Edge cases to handle up front

- **Worktree HEAD differs from `session.HeadSHA`** — gopls sees disk state, which may not match the diff. Detect via `git rev-parse HEAD` at TUI start; if mismatched, status bar warning: "LSP results may be stale: worktree HEAD differs from session snapshot". Don't disable the feature; just warn.
- **gopls not installed / slow init** — first `gd` press shows "initializing gopls…" (can take 10–60s on cold projects). Subsequent requests are fast. Make sure `Init` doesn't block the UI.
- **LSP subprocess crashes** — `Manager` detects exit on its goroutine, drops the cached client, and next request respawns.
- **Definition result outside worktree** (e.g., stdlib, `$GOPATH/pkg/mod`) — still open read-only in the out-of-diff viewer; label as "external". Don't try to set `fileIdx`.

## Critical files to modify / create

**Create**:
- `lsp/jsonrpc.go`, `lsp/types.go`, `lsp/client.go`, `lsp/server.go`, `lsp/registry.go`
- `lsp/client_test.go`, `lsp/registry_test.go`
- `diff/tui/definition.go` — new file holding `goToDefinition()`, `defResultMsg`, jump-history helpers, and `ModeDefinition` rendering (keeps `model.go` from ballooning).

**Modify**:
- `diff/tui/model.go` — add `focusedCol`, `worktreePath`, `lspManager`, `jumpHistory`, `lastKey`, handle new key bindings, new `ModeDefinition` branch in `Update`/`View`.
- `diff/tui/keymap.go` — new bindings.
- `diff/tui/render.go` — column cursor rendering; in-diff reverse-lookup index in `rebuildDiffView`; `ModeDefinition` rendering; help modal entries.
- `diff/tui/view.go` — thread `worktreePath` into `Run`/`New`.
- `diff/resolve.go` — have `Resolve` return workspace dir alongside session+diff.
- `cli/diff.go` — pass through as needed.
- `go.mod` — no new deps.

## Reusable patterns we're leaning on

- `runGH` pattern (`diff/gh.go:84-92`) → model for `lsp/client.go` subprocess spawn.
- `fakeGH` test pattern (`diff/gh_test.go:14-31`) → fake LSP server script for `lsp/client_test.go`.
- `listWorktrees` (`diff/worktree.go:30-40`) → worktree lookup by branch for `-s` path.
- `renderInspectModal` (`render.go:360-405`) + `inspectHunkVP` two-pane layout → model for `ModeDefinition` view.
- `highlightLine` (`render.go:462-490`) → reuse directly for out-of-diff file rendering.
- `pickerModel` (`diff/tui/picker.go`) → reuse for multi-location definition picker.

## Verification

1. Build: `go build ./...` and `go vet ./...`.
2. Unit tests: `go test ./lsp/...` — fake LSP server script exercises initialize/didOpen/definition roundtrip.
3. Integration (manual) on sigil itself (it's a Go project with `go.mod`):
   - `sigil diff` inside this repo against a test PR that edits `diff/tui/model.go`.
   - On a line calling `m.rebuildDiffView()`, press `w` until cursor is on `rebuildDiffView`, then `gd` → expect jump to its definition lower in the file (in-diff case, if the hunk includes it, otherwise out-of-diff).
   - On a line calling `diff.MarkOrphans(...)`, `gd` → expect `ModeDefinition` opening `diff/anchor.go` at `MarkOrphans`.
   - Press `ctrl+o` → expect return to original diff position.
   - Press `gd` in whitespace → status bar message, no mode change.
4. Stale-HEAD verification: check out a different commit in the worktree, relaunch `sigil diff -s <id>` → expect stale-snapshot warning.
5. Negative test: rename `gopls` off `PATH`, press `gd` → expect "gopls: executable not found" in status bar, no crash.
6. No TypeScript/Python work is in this plan, but `lsp.Registry` adds them with a one-line map entry when we're ready.

## Work streams (for epic decomposition)

This plan becomes a `tk` epic. Streams are ordered by dependency, not by importance.

1. **`lsp/` package** — JSON-RPC codec, narrow LSP types, `Client`, `Manager`, registry, fake-server tests. No in-repo deps; can start immediately.
2. **Worktree plumbing** — `diff.Resolve` returns workspace dir; `tui.Run` / `New` accept it; stale-HEAD detection + warning. Small, independent; can run in parallel with (1) and (3).
3. **Column cursor + vi motions** — `focusedCol`, `h/l/w/b/e`, word-boundary scanner, focused-rune rendering. Independent; can run in parallel with (1) and (2).
4. **Go-to-definition action** — `gd` / `ctrl+]`, symbol extraction, async `tea.Cmd`, `defResultMsg`, in-diff reverse-lookup index, jump-history stack. **Blocked by 1, 2, 3.**
5. **`ModeDefinition` viewer** — full-file read-only mode using `highlightLine` + `viewport`, scroll-to-target, recursion via `gd`. **Blocked by 2.** Can run in parallel with (4) once (2) lands.
6. **Keymap / help / status polish** — new bindings in `keymap.go`, help modal entries, key bar entries. **Blocked by 3, 4, 5** (cosmetic tail).
7. **Manual QA + docs** — run through the Verification steps; update `README` / `cli/install_skill.go` if reviewer-facing behavior changed. **Blocked by all above.**

<!--
@review-backmatter

"0001":
  offset: 1
  span: 3
  comment: "is JSON-RPC common for interacting with LSPs? can we chose to do something like openapi / swagger?"
  status: resolved

-->
