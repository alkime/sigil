package lsp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeLSP writes a shell script to a temp dir and returns its absolute path.
// Mirrors the fakeGH pattern in diff/gh_test.go.
func fakeLSP(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-lsp")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lsp: %v", err)
	}
	return path
}

// roundtripScript is a minimal LSP server that handles initialize, initialized,
// didOpen, textDocument/definition, shutdown, and exit. It parses the id out of
// each request and echoes it in the response so any id sequence works.
const roundtripScript = `#!/bin/sh
send() {
    body="$1"
    len=$(printf %s "$body" | wc -c | tr -d ' ')
    printf 'Content-Length: %s\r\n\r\n%s' "$len" "$body"
}

read_msg() {
    cl=0
    while IFS= read -r line; do
        line=$(printf %s "$line" | tr -d '\r')
        [ -z "$line" ] && break
        case "$line" in
            "Content-Length: "*) cl=${line#Content-Length: } ;;
        esac
    done
    BODY=""
    ID=""
    if [ "$cl" -gt 0 ]; then
        BODY=$(dd bs=1 count="$cl" 2>/dev/null)
        ID=$(printf %s "$BODY" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')
    fi
}

read_msg
send "{\"jsonrpc\":\"2.0\",\"id\":$ID,\"result\":{\"capabilities\":{\"definitionProvider\":true}}}"

while :; do
    read_msg
    if [ -z "$BODY" ]; then
        exit 0
    fi
    case "$BODY" in
        *'"method":"initialized"'*) ;;
        *'"method":"textDocument/didOpen"'*) ;;
        *'"method":"textDocument/definition"'*)
            send "{\"jsonrpc\":\"2.0\",\"id\":$ID,\"result\":[{\"uri\":\"file:///tmp/def.go\",\"range\":{\"start\":{\"line\":10,\"character\":5},\"end\":{\"line\":10,\"character\":10}}}]}"
            ;;
        *'"method":"shutdown"'*)
            send "{\"jsonrpc\":\"2.0\",\"id\":$ID,\"result\":null}"
            ;;
        *'"method":"exit"'*)
            exit 0
            ;;
    esac
done
`

// hangOnDefinitionScript initializes normally but never answers a definition
// request. On stdin close (triggered by Client.Close) it exits cleanly.
const hangOnDefinitionScript = `#!/bin/sh
send() {
    body="$1"
    len=$(printf %s "$body" | wc -c | tr -d ' ')
    printf 'Content-Length: %s\r\n\r\n%s' "$len" "$body"
}

read_msg() {
    cl=0
    while IFS= read -r line; do
        line=$(printf %s "$line" | tr -d '\r')
        [ -z "$line" ] && break
        case "$line" in
            "Content-Length: "*) cl=${line#Content-Length: } ;;
        esac
    done
    BODY=""
    ID=""
    if [ "$cl" -gt 0 ]; then
        BODY=$(dd bs=1 count="$cl" 2>/dev/null)
        ID=$(printf %s "$BODY" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')
    fi
}

read_msg
send "{\"jsonrpc\":\"2.0\",\"id\":$ID,\"result\":{\"capabilities\":{\"definitionProvider\":true}}}"

while :; do
    read_msg
    if [ -z "$BODY" ]; then
        exit 0
    fi
    case "$BODY" in
        *'"method":"textDocument/definition"'*)
            # Drain stdin until the client closes it; then exit.
            cat > /dev/null
            exit 0
            ;;
        *'"method":"exit"'*)
            exit 0
            ;;
    esac
done
`

func TestClient_DefinitionRoundtrip(t *testing.T) {
	bin := fakeLSP(t, roundtripScript)

	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := NewClient(ctx, ServerConfig{Language: "go", Binary: bin}, dir)
	if err != nil {
		t.Fatalf("NewClient: %v\nstderr:\n%s", err, stderrOrEmpty(client))
	}
	t.Cleanup(func() { _ = client.Close() })

	locs, err := client.Definition(ctx, src, 2, 5)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("got %d locations, want 1: %+v", len(locs), locs)
	}
	got := locs[0]
	if got.URI != "file:///tmp/def.go" {
		t.Errorf("URI = %q, want %q", got.URI, "file:///tmp/def.go")
	}
	if got.Range.Start != (Position{Line: 10, Character: 5}) {
		t.Errorf("Start = %+v, want {10 5}", got.Range.Start)
	}
	if got.Range.End != (Position{Line: 10, Character: 10}) {
		t.Errorf("End = %+v, want {10 10}", got.Range.End)
	}
}

func TestClient_DidOpenSentOnce(t *testing.T) {
	// Two Definition calls on the same file should share a single didOpen.
	// The roundtrip script happily responds to every definition request,
	// so successful completion of two calls exercises the cache path.
	bin := fakeLSP(t, roundtripScript)
	dir := t.TempDir()
	src := filepath.Join(dir, "a.go")
	if err := os.WriteFile(src, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := NewClient(ctx, ServerConfig{Language: "go", Binary: bin}, dir)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	for i := 0; i < 2; i++ {
		if _, err := client.Definition(ctx, src, 0, 0); err != nil {
			t.Fatalf("Definition #%d: %v", i, err)
		}
	}
	client.mu.Lock()
	opened := len(client.opened)
	client.mu.Unlock()
	if opened != 1 {
		t.Errorf("opened cache size = %d, want 1", opened)
	}
}

func TestClient_DefinitionContextCancel(t *testing.T) {
	bin := fakeLSP(t, hangOnDefinitionScript)
	dir := t.TempDir()
	src := filepath.Join(dir, "x.go")
	if err := os.WriteFile(src, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := NewClient(ctx, ServerConfig{Language: "go", Binary: bin}, dir)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	defCtx, defCancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		_, err := client.Definition(defCtx, src, 0, 0)
		errCh <- err
	}()
	time.Sleep(150 * time.Millisecond)
	defCancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Definition err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Definition did not return within 2s of cancel")
	}
}

func TestClient_CloseIsIdempotent(t *testing.T) {
	bin := fakeLSP(t, roundtripScript)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := NewClient(ctx, ServerConfig{Language: "go", Binary: bin}, t.TempDir())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestClient_MissingBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := NewClient(ctx, ServerConfig{Language: "x", Binary: "/does/not/exist/definitely"}, t.TempDir())
	if err == nil {
		t.Fatal("NewClient: err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "lsp") {
		t.Errorf("err = %v, want lsp-prefixed", err)
	}
}

func TestManager_CachesByRoot(t *testing.T) {
	bin := fakeLSP(t, roundtripScript)
	cfg := ServerConfig{Language: "go", Binary: bin}
	root := t.TempDir()

	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c1, err := m.Get(ctx, cfg, root)
	if err != nil {
		t.Fatalf("Get #1: %v", err)
	}
	c2, err := m.Get(ctx, cfg, root)
	if err != nil {
		t.Fatalf("Get #2: %v", err)
	}
	if c1 != c2 {
		t.Errorf("Get returned different clients for same root; want the same cached client")
	}
}

func TestManager_DifferentRootsSpawnSeparateClients(t *testing.T) {
	bin := fakeLSP(t, roundtripScript)
	cfg := ServerConfig{Language: "go", Binary: bin}

	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c1, err := m.Get(ctx, cfg, t.TempDir())
	if err != nil {
		t.Fatalf("Get #1: %v", err)
	}
	c2, err := m.Get(ctx, cfg, t.TempDir())
	if err != nil {
		t.Fatalf("Get #2: %v", err)
	}
	if c1 == c2 {
		t.Errorf("Get returned the same client for different roots; want distinct")
	}
}

func TestPathURIRoundtrip(t *testing.T) {
	cases := []string{
		"/tmp/foo.go",
		"/Users/jane/dev/sigil/lsp/client.go",
	}
	for _, p := range cases {
		got := URIToPath(pathToURI(p))
		if got != p {
			t.Errorf("roundtrip(%q) = %q", p, got)
		}
	}
}

func TestParseDefinitionResult(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantLen int
	}{
		{"null", `null`, 0},
		{"empty array", `[]`, 0},
		{"single location object", `{"uri":"file:///x","range":{"start":{"line":1,"character":2},"end":{"line":1,"character":3}}}`, 1},
		{"location array", `[{"uri":"file:///x","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			locs, err := parseDefinitionResult([]byte(tc.input))
			if err != nil {
				t.Fatalf("parseDefinitionResult: %v", err)
			}
			if len(locs) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(locs), tc.wantLen)
			}
		})
	}
}

// stderrOrEmpty safely reads Stderr even if c is nil (e.g., NewClient returned nil).
func stderrOrEmpty(c *Client) string {
	if c == nil {
		return ""
	}
	return c.Stderr()
}
