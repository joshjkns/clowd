package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

// The language server protocol is JSON-RPC 2.0 over a byte stream, each
// message preceded by a Content-Length header. It is small enough to
// implement here rather than add a dependency.

// message is any JSON-RPC message: a request has an ID and a method, a
// notification only a method, and a response an ID and a result or error.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// isRequest reports whether m expects a response.
func (m *message) isRequest() bool { return m.Method != "" && len(m.ID) > 0 }

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return e.Message }

// Error codes from the JSON-RPC and LSP specifications.
const (
	codeParseError           = -32700
	codeInvalidParams        = -32602
	codeMethodNotFound       = -32601
	codeServerNotInitialized = -32002
	codeInvalidRequest       = -32600
)

// conn reads and writes framed messages.
type conn struct {
	r *bufio.Reader

	mu sync.Mutex // serialises writes
	w  io.Writer
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w}
}

// read returns the next message. It returns io.EOF when the stream ends
// between messages.
func (c *conn) read() (*message, error) {
	h, err := textproto.NewReader(c.r).ReadMIMEHeader()
	if err != nil {
		if err == io.EOF && len(h) == 0 {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("reading header: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(h.Get("Content-Length")))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("bad Content-Length %q", h.Get("Content-Length"))
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	m := &message{}
	if err := json.Unmarshal(body, m); err != nil {
		// Still a message: the caller answers it with a parse error.
		return &message{Error: &rpcError{Code: codeParseError, Message: err.Error()}}, nil
	}
	return m, nil
}

func (c *conn) write(m *message) error {
	m.JSONRPC = "2.0"
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

// reply answers the request with id. A nil result is sent as JSON null,
// which LSP uses for "nothing here".
func (c *conn) reply(id json.RawMessage, result any, rerr *rpcError) error {
	m := &message{ID: id, Error: rerr}
	if rerr == nil {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		m.Result = b
	}
	return c.write(m)
}

// notify sends a notification.
func (c *conn) notify(method string, params any) error {
	b, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(&message{Method: method, Params: b})
}
