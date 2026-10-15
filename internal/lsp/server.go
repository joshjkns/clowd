// Package lsp is Clowd's language server, run as `clowd lsp` (05-tooling.md).
//
// It speaks LSP over stdin and stdout and is backed by the same packages
// as the CLI: diagnostics are exactly what `clowd check` reports, and
// formatting is `clowd fmt`. Requests are handled one at a time, in order;
// a whole file is re-checked on every change, which is fast enough for
// files people write by hand.
package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"

	"github.com/joshjkns/clowd/internal/analyze"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/format"
)

// Serve runs the server until the client sends `exit`. It returns an
// error if the stream fails, or if the client exits or disconnects
// without asking the server to shut down first.
func Serve(r io.Reader, w io.Writer, version string) error {
	s := &server{conn: newConn(r, w), version: version, enc: utf16Encoding, docs: map[string]*document{}}
	for {
		m, err := s.conn.read()
		if err == io.EOF {
			if s.shutdown {
				return nil
			}
			return errors.New("the client disconnected without shutting the server down")
		}
		if err != nil {
			return err
		}
		if m.Method == "exit" {
			if s.shutdown {
				return nil
			}
			return errors.New("the client exited without shutting the server down")
		}
		if err := s.handle(m); err != nil {
			return err
		}
	}
}

type server struct {
	conn    *conn
	version string
	enc     encoding
	docs    map[string]*document
	// initialized is set by `initialize`, and shutdown by `shutdown`.
	initialized, shutdown bool
}

// document is an open file and its latest analysis.
type document struct {
	uri      string
	version  int
	file     *diag.File
	analysis *analyze.Analysis
}

// handle answers one message. Only a failure to write is returned:
// everything else is reported to the client.
func (s *server) handle(m *message) error {
	if m.Error != nil && m.Method == "" && len(m.ID) == 0 {
		return s.conn.reply(json.RawMessage("null"), nil, m.Error) // unparsable
	}
	if m.Method == "" {
		return nil // a response; the server sends no requests
	}
	result, rerr := s.dispatch(m)
	if !m.isRequest() {
		return nil
	}
	return s.conn.reply(m.ID, result, rerr)
}

// dispatch runs the handler for m. Notifications' results are dropped.
func (s *server) dispatch(m *message) (any, *rpcError) {
	switch {
	case m.Method == "initialize":
		return s.initialize(m.Params)
	case !s.initialized:
		return nil, &rpcError{Code: codeServerNotInitialized, Message: "the server has not been initialized"}
	case s.shutdown:
		return nil, &rpcError{Code: codeInvalidRequest, Message: "the server is shutting down"}
	}
	switch m.Method {
	case "initialized":
		return nil, nil
	case "shutdown":
		s.shutdown = true
		return nil, nil
	case "textDocument/didOpen":
		var p didOpenParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return nil, s.open(p.TextDocument.URI, p.TextDocument.Version, p.TextDocument.Text)
	case "textDocument/didChange":
		var p didChangeParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		if len(p.ContentChanges) == 0 {
			return nil, nil
		}
		return nil, s.open(p.TextDocument.URI, p.TextDocument.Version, p.ContentChanges[len(p.ContentChanges)-1].Text)
	case "textDocument/didClose":
		var p didCloseParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		delete(s.docs, p.TextDocument.URI)
		// Clear the closed file's problems from the editor's list.
		if err := s.conn.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{URI: p.TextDocument.URI, Diagnostics: []diagnostic{}}); err != nil {
			return nil, &rpcError{Code: codeInvalidRequest, Message: err.Error()}
		}
		return nil, nil
	case "textDocument/hover":
		doc, pos, rerr := s.at(m.Params)
		if rerr != nil || doc == nil {
			return nil, rerr
		}
		return s.hover(doc, pos), nil
	case "textDocument/completion":
		doc, pos, rerr := s.at(m.Params)
		if rerr != nil || doc == nil {
			return nil, rerr
		}
		return s.complete(doc, pos), nil
	case "textDocument/formatting":
		var p documentFormattingParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		doc := s.docs[p.TextDocument.URI]
		if doc == nil {
			return nil, nil
		}
		return s.format(doc), nil
	case "textDocument/semanticTokens/full":
		var p semanticTokensParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		doc := s.docs[p.TextDocument.URI]
		if doc == nil {
			return nil, nil
		}
		return s.semanticTokens(doc), nil
	}
	if m.isRequest() {
		return nil, &rpcError{Code: codeMethodNotFound, Message: fmt.Sprintf("%s is not supported", m.Method)}
	}
	return nil, nil // unknown notifications, such as $/cancelRequest, are ignored
}

func invalidParams(err error) *rpcError {
	return &rpcError{Code: codeInvalidParams, Message: err.Error()}
}

func (s *server) initialize(params json.RawMessage) (any, *rpcError) {
	if s.initialized {
		return nil, &rpcError{Code: codeInvalidRequest, Message: "the server is already initialized"}
	}
	var p initializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, invalidParams(err)
	}
	for _, e := range p.Capabilities.General.PositionEncodings {
		if encoding(e) == utf8Encoding {
			s.enc = utf8Encoding // no conversion needed: spans are bytes
		}
	}
	s.initialized = true
	return initializeResult{
		Capabilities: serverCapabilities{
			PositionEncoding:           string(s.enc),
			TextDocumentSync:           textDocumentSync{OpenClose: true, Change: 1},
			HoverProvider:              true,
			CompletionProvider:         completionProvider{TriggerCharacters: []string{" "}},
			DocumentFormattingProvider: true,
			SemanticTokensProvider: semanticTokensOptions{
				Legend: semanticTokensLegend{TokenTypes: tokenTypes, TokenModifiers: tokenModifiers},
				Full:   true,
			},
		},
		ServerInfo: serverInfo{Name: "clowd", Version: s.version},
	}, nil
}

// at decodes a position request. The document is nil if it isn't open.
func (s *server) at(params json.RawMessage) (*document, int, *rpcError) {
	var p textDocumentPositionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, 0, invalidParams(err)
	}
	doc := s.docs[p.TextDocument.URI]
	if doc == nil {
		return nil, 0, nil
	}
	return doc, toOffset(doc.file, p.Position, s.enc), nil
}

// open stores a document's new text, checks it and publishes its
// diagnostics.
func (s *server) open(uri string, version int, text string) *rpcError {
	doc := &document{uri: uri, version: version, file: diag.NewFile(fileName(uri), text)}
	doc.analysis = analyze.File(doc.file, nil)
	s.docs[uri] = doc
	if err := s.conn.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Version:     &doc.version,
		Diagnostics: s.diagnostics(doc),
	}); err != nil {
		return &rpcError{Code: codeInvalidRequest, Message: err.Error()}
	}
	return nil
}

// fileName is the name diagnostics and lowering use for uri: its base
// name, as `clowd check app.clowd` would show it.
func fileName(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Path != "" {
		return filepath.Base(filepath.FromSlash(u.Path))
	}
	return uri
}

func (s *server) diagnostics(doc *document) []diagnostic {
	out := []diagnostic{}
	for _, d := range doc.analysis.All {
		msg := d.Message
		if d.Suggestion != "" {
			msg += "\nhelp: " + d.Suggestion
		}
		out = append(out, diagnostic{
			Range:    toRange(doc.file, d.Span, s.enc),
			Severity: map[diag.Severity]int{diag.Error: severityError, diag.Warning: severityWarning, diag.Info: severityInformation}[d.Severity],
			Source:   "clowd",
			Message:  msg,
		})
	}
	return out
}

// format returns the edits that format doc: none when it is already
// formatted or has syntax errors, which the formatter refuses (D11), or
// one edit replacing the whole text.
func (s *server) format(doc *document) []textEdit {
	out, _ := format.File(doc.file)
	if out == nil || string(out) == doc.file.Text {
		return []textEdit{}
	}
	return []textEdit{{
		Range:   lspRange{Start: position{}, End: toPosition(doc.file, len(doc.file.Text), s.enc)},
		NewText: string(out),
	}}
}
