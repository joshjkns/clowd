package lsp

// The subset of the LSP 3.17 types the server uses.

type initializeParams struct {
	Capabilities struct {
		General struct {
			PositionEncodings []string `json:"positionEncodings"`
		} `json:"general"`
	} `json:"capabilities"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type serverCapabilities struct {
	PositionEncoding           string             `json:"positionEncoding"`
	TextDocumentSync           textDocumentSync   `json:"textDocumentSync"`
	HoverProvider              bool               `json:"hoverProvider"`
	CompletionProvider         completionProvider `json:"completionProvider"`
	DocumentFormattingProvider bool               `json:"documentFormattingProvider"`
}

type textDocumentSync struct {
	OpenClose bool `json:"openClose"`
	// Change is 1 for full-text sync: Clowd files are small, and the
	// whole file is re-checked on every change anyway.
	Change int `json:"change"`
}

type completionProvider struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentItem struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
	Text    string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument struct {
		URI     string `json:"uri"`
		Version int    `json:"version"`
	} `json:"textDocument"`
	// With full sync each change is the whole text; the last one wins.
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type documentFormattingParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// Diagnostic severities.
const (
	severityError       = 1
	severityWarning     = 2
	severityInformation = 3
)

type diagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hover struct {
	Contents markupContent `json:"contents"`
	Range    *lspRange     `json:"range,omitempty"`
}

// Completion item kinds.
const (
	itemField      = 5
	itemVariable   = 6
	itemClass      = 7
	itemModule     = 9
	itemProperty   = 10
	itemKeyword    = 14
	itemEnumMember = 20
)

type completionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind,omitempty"`
	Detail        string         `json:"detail,omitempty"`
	Documentation *markupContent `json:"documentation,omitempty"`
	TextEdit      *textEdit      `json:"textEdit,omitempty"`
	// SortText keeps the server's order; clients sort by label otherwise.
	SortText string `json:"sortText,omitempty"`
}

type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}
