package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// message is a JSON-RPC 2.0 envelope. Requests set ID and Method; notifications
// set Method only; responses set ID plus Result or Error.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("jsonrpc: %d %s", e.Code, e.Message)
}

// codec frames JSON-RPC 2.0 messages with LSP Content-Length headers.
type codec struct {
	r  *bufio.Reader
	w  io.Writer
	wm sync.Mutex
}

func newCodec(r io.Reader, w io.Writer) *codec {
	return &codec{r: bufio.NewReader(r), w: w}
}

func (c *codec) write(msg *message) error {
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	c.wm.Lock()
	defer c.wm.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = c.w.Write(data)
	return err
}

func (c *codec) read() (*message, error) {
	contentLength := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("jsonrpc: invalid header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("jsonrpc: bad Content-Length: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("jsonrpc: missing Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, err
	}
	var msg message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("jsonrpc: decode body: %w", err)
	}
	return &msg, nil
}
