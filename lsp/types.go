package lsp

// Position is a 0-indexed (line, character) pair per the LSP spec.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a half-open span [Start, End) in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a file URI + range; the result type for textDocument/definition.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a document by URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentItem is a document with full contents; used by didOpen.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidOpenTextDocumentParams is the payload for textDocument/didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DefinitionParams is the payload for textDocument/definition.
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// InitializeParams is the narrow subset of the LSP initialize payload we send.
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri,omitempty"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities advertises what the client supports.
type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// TextDocumentClientCapabilities is the textDocument branch we populate.
type TextDocumentClientCapabilities struct {
	Definition *DefinitionClientCapabilities `json:"definition,omitempty"`
}

// DefinitionClientCapabilities configures go-to-definition features.
type DefinitionClientCapabilities struct {
	LinkSupport bool `json:"linkSupport,omitempty"`
}
