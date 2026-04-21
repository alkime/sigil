---
id: sig-4iuw
status: open
deps: []
links: []
created: 2026-04-21T00:08:53Z
type: task
priority: 2
assignee: James McKernan
parent: sig-8n5o
tags: [sigil, lsp]
---
# LSP package: JSON-RPC client + narrow types + Manager + registry

Build lsp/ package: Content-Length-framed JSON-RPC 2.0 codec, narrow LSP structs (InitializeParams, Position, Location, TextDocumentIdentifier, DefinitionParams), Client with handshake + didOpen cache + Definition RPC, Manager keyed on (language, project root), ServerConfig + registry (.go → gopls). Self-contained; no deps on other sigil packages.

## Design

Files to create:
- lsp/jsonrpc.go — Content-Length framed codec over io.ReadWriteCloser
- lsp/types.go — only structs we need
- lsp/client.go — Client: spawn, handshake (initialize + initialized), request/response correlation via map[id]chan resp with mutex, auto-didOpen on first file use, Close = shutdown + exit + wait + kill
- lsp/server.go — ServerConfig{Language, Binary, Args, RootMarkers}
- lsp/registry.go — ForExtension(ext) ServerConfig; defaults map {'.go': gopls}
- lsp/manager.go — Manager.Get(ctx, cfg, root) *Client with caching; Manager.Close()
- lsp/client_test.go — fake LSP server shell script mirroring diff/gh_test.go fakeGH pattern
- lsp/registry_test.go

Public API (v1):
- Client.Definition(ctx, absFile string, line, col int) ([]Location, error)
- Location{URI string, Range Range}, Range{Start, End Position}, Position{Line, Character int} — 0-indexed, LSP convention.

Reuse: runGH pattern at diff/gh.go:84-92 for subprocess spawn; fakeGH pattern at diff/gh_test.go:14-31 for tests.

## Acceptance Criteria

- go build ./lsp/... succeeds.
- go test ./lsp/... passes, including a fake-server roundtrip that covers initialize → initialized → textDocument/didOpen → textDocument/definition → shutdown → exit.
- Client.Close() cleanly terminates the subprocess (no zombies under go test -race).
- Context cancellation aborts an in-flight Definition call within a reasonable window.
- Registry.ForExtension('.go') returns gopls config; unknown extension returns ok=false.
- Manager caches clients by (language, root); two calls with same root share one subprocess.

