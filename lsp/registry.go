package lsp

var defaultServers = map[string]ServerConfig{
	".go": {
		Language:    "go",
		Binary:      "gopls",
		Args:        nil,
		RootMarkers: []string{"go.mod", "go.work"},
	},
}

// ForExtension returns the default server config for a file extension
// (e.g. ".go"). The second return reports whether a server is registered.
func ForExtension(ext string) (ServerConfig, bool) {
	cfg, ok := defaultServers[ext]
	return cfg, ok
}
