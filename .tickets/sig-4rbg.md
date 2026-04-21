---
id: sig-4rbg
status: closed
deps: [sig-4iuw, sig-7qra, sig-pamc]
links: []
created: 2026-04-21T00:09:50Z
type: task
priority: 2
assignee: James McKernan
parent: sig-8n5o
tags: [sigil, diff, lsp, tui]
---
# Go-to-definition action + async tea.Cmd + jump history

Wire the user-facing go-to-definition action: gd (chord) and ctrl+] (single-key alias) extract the symbol at the cursor, send an async LSP definition request, and route the response to either an in-diff scroll or the out-of-diff ModeDefinition viewer (stream 5). Includes cancellation of in-flight requests and a bounded jump history with ctrl+o.

## Design

New file: diff/tui/definition.go — keeps model.go from ballooning.

Flow (goToDefinition on keypress):
1. Read m.linesMeta[m.focusedLine]; reject hunk-header / comment-marker.
2. Use WordAt (from stream 3) on hunk.Lines[meta.lineInHunk].Text at focusedCol. If no word, status 'no symbol under cursor'; return.
3. Map to disk: side := lineKindToSide(meta.lineKind). Add/Context → file.NewPath + line.NewLineNum. Delete → status 'go-to-def on deleted lines not supported'; return.
4. absFile := filepath.Join(worktreePath, file.NewPath). lspLine := NewLineNum - 1. lspCol := rune index within raw line text at focusedCol.
5. cfg, ok := lsp.ForExtension(filepath.Ext(absFile)); !ok → status 'no LSP configured for .xyz'.
6. projectRoot: walk up from absFile looking for cfg.RootMarkers; fall back to worktreePath.
7. Return tea.Cmd that calls manager.Get(ctx, cfg, root).Definition(ctx, absFile, lspLine, lspCol) and emits defResultMsg{sym, locations, err}.
8. Model keeps lspReqCancel context.CancelFunc; new gd press cancels the previous in-flight request.
9. Show 'resolving definition...' status immediately on gd.

Chord handling: Model gains lastKey string. updateNormal on 'g': if lastKey=='g', run goToDefinition and clear; else record lastKey='g'. Any other key clears lastKey (except sequences we ever add later). Ctrl+] is the single-key alternative.

defResultMsg handler:
- err or zero locations → status bar error.
- One location → in-diff vs out-of-diff dispatch (see below).
- Multiple → reuse pickerModel (diff/tui/picker.go) to choose.

In-diff reverse lookup: during rebuildDiffView, build m.lineIndex map[string]map[int32]int keyed by NewPath → NewLineNum → rendered line index. Lookup is O(1) per definition.

In-diff dispatch:
- Push jumpEntry{fileIdx, focusedLine, focusedCol, mode} onto m.jumpHistory (cap 32, drop oldest).
- Set m.fileIdx, rebuildDiffView(), focusedLine = idx, focusedCol = 0, ensureLineVisible().

Out-of-diff dispatch: delegate to stream 5 (OpenDefinition(location)).

Jump history (ctrl+o): pop entry; restore mode + fileIdx + focusedLine + focusedCol; ensureLineVisible. ctrl+i (redo) deferred.

Reuse: pickerModel in diff/tui/picker.go; listWorktrees not needed here (stream 2 handles worktree path).

## Acceptance Criteria

- gd on an in-diff symbol (same file, another hunk) scrolls the viewport to the definition and highlights the line.
- gd on an out-of-diff symbol triggers stream 5's ModeDefinition (verified once stream 5 lands).
- gd in whitespace / punctuation shows 'no symbol under cursor', no mode change.
- gd on a delete-side line shows a clean not-supported message.
- ctrl+] is an accepted alias.
- ctrl+o restores the previous location (file + line + col).
- Rapid gd presses cancel the prior request (no hung UI).
- Status bar shows 'resolving definition...' during the request.
- Missing gopls shows a clean error (not a panic).

