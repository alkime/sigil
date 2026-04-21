package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrClosed is returned when a method is called on a closed Client.
var ErrClosed = errors.New("lsp: client closed")

// Client is a single language-server subprocess plus its JSON-RPC state.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *bytes.Buffer
	codec  *codec

	cfg  ServerConfig
	root string

	nextID atomic.Int64

	mu      sync.Mutex
	pending map[int64]chan *message
	opened  map[string]bool
	closed  bool

	done chan struct{}
}

// NewClient spawns the configured language server and completes the initialize
// handshake. Callers own the returned Client and must call Close.
func NewClient(ctx context.Context, cfg ServerConfig, root string) (*Client, error) {
	if cfg.Binary == "" {
		return nil, fmt.Errorf("lsp: empty Binary in ServerConfig")
	}
	cmd := exec.Command(cfg.Binary, cfg.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdout pipe: %w", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp: start %s: %w", cfg.Binary, err)
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		codec:   newCodec(stdout, stdin),
		cfg:     cfg,
		root:    root,
		pending: map[int64]chan *message{},
		opened:  map[string]bool{},
		done:    make(chan struct{}),
	}
	go c.readLoop()

	if err := c.initialize(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) readLoop() {
	defer close(c.done)
	for {
		msg, err := c.codec.read()
		if err != nil {
			c.mu.Lock()
			for id, ch := range c.pending {
				delete(c.pending, id)
				close(ch)
			}
			c.mu.Unlock()
			return
		}
		if msg.ID == nil {
			// Server notification (logMessage, publishDiagnostics, etc.). Ignore in v1.
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[*msg.ID]
		if ok {
			delete(c.pending, *msg.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- msg
		}
	}
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("lsp: marshal %s params: %w", method, err)
		}
		raw = b
	}

	id := c.nextID.Add(1)
	ch := make(chan *message, 1)

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.codec.write(&message{JSONRPC: "2.0", ID: &id, Method: method, Params: raw}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("lsp: write %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("lsp: connection closed during %s", method)
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (c *Client) notify(method string, params any) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("lsp: marshal %s params: %w", method, err)
		}
		raw = b
	}
	return c.codec.write(&message{JSONRPC: "2.0", Method: method, Params: raw})
}

func (c *Client) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   pathToURI(c.root),
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				Definition: &DefinitionClientCapabilities{LinkSupport: false},
			},
		},
	}
	if _, err := c.call(ctx, "initialize", params); err != nil {
		return fmt.Errorf("lsp: initialize: %w", err)
	}
	if err := c.notify("initialized", struct{}{}); err != nil {
		return fmt.Errorf("lsp: initialized: %w", err)
	}
	return nil
}

// Definition resolves the definition of the symbol at (line, col) in absFile.
// line and col are 0-indexed per LSP convention.
func (c *Client) Definition(ctx context.Context, absFile string, line, col int) ([]Location, error) {
	if err := c.ensureOpen(absFile); err != nil {
		return nil, err
	}
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(absFile)},
		Position:     Position{Line: line, Character: col},
	}
	raw, err := c.call(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}
	return parseDefinitionResult(raw)
}

// parseDefinitionResult decodes the LSP definition response, which may be null,
// a single Location, or an array of Locations.
func parseDefinitionResult(raw json.RawMessage) ([]Location, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var locs []Location
		if err := json.Unmarshal(trimmed, &locs); err != nil {
			return nil, fmt.Errorf("lsp: decode locations: %w", err)
		}
		return locs, nil
	}
	var loc Location
	if err := json.Unmarshal(trimmed, &loc); err != nil {
		return nil, fmt.Errorf("lsp: decode location: %w", err)
	}
	return []Location{loc}, nil
}

func (c *Client) ensureOpen(absFile string) error {
	c.mu.Lock()
	if c.opened[absFile] {
		c.mu.Unlock()
		return nil
	}
	c.opened[absFile] = true
	c.mu.Unlock()

	data, err := os.ReadFile(absFile)
	if err != nil {
		c.mu.Lock()
		delete(c.opened, absFile)
		c.mu.Unlock()
		return fmt.Errorf("lsp: read %s: %w", absFile, err)
	}
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        pathToURI(absFile),
			LanguageID: c.cfg.Language,
			Version:    1,
			Text:       string(data),
		},
	}
	if err := c.notify("textDocument/didOpen", params); err != nil {
		c.mu.Lock()
		delete(c.opened, absFile)
		c.mu.Unlock()
		return err
	}
	return nil
}

// Close performs the LSP shutdown/exit handshake and terminates the subprocess.
// It is safe to call Close more than once.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, _ = c.call(shutdownCtx, "shutdown", nil)
	cancel()
	_ = c.notify("exit", nil)

	_ = c.stdin.Close()

	exited := make(chan error, 1)
	go func() { exited <- c.cmd.Wait() }()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		_ = c.cmd.Process.Kill()
		<-exited
	}
	<-c.done
	return nil
}

// Stderr returns a snapshot of everything the server has written to stderr.
// Intended for error diagnostics after a failure.
func (c *Client) Stderr() string {
	return c.stderr.String()
}

// pathToURI converts an absolute filesystem path to a file:// URI.
func pathToURI(p string) string {
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := &url.URL{Scheme: "file", Path: p}
	return u.String()
}

// URIToPath converts a file:// URI to a filesystem path.
func URIToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	return u.Path
}
