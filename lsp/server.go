package lsp

// ServerConfig describes how to launch a language server.
type ServerConfig struct {
	// Language is the LSP languageId used in didOpen (e.g. "go").
	Language string
	// Binary is the executable name or absolute path.
	Binary string
	// Args are extra arguments passed to the binary.
	Args []string
	// RootMarkers are filenames that mark a project root when walking up from a file (e.g. "go.mod").
	RootMarkers []string
}
