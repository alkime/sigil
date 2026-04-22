package lsp

var defaultServers = map[string]ServerConfig{
	".go": {
		Language:    "go",
		Binary:      "gopls",
		Args:        nil,
		RootMarkers: []string{"go.mod", "go.work"},
	},
	".ts": {
		Language:    "typescript",
		Binary:      "typescript-language-server",
		Args:        []string{"--stdio"},
		RootMarkers: []string{"tsconfig.json", "jsconfig.json", "package.json"},
	},
	".tsx": {
		Language:    "typescriptreact",
		Binary:      "typescript-language-server",
		Args:        []string{"--stdio"},
		RootMarkers: []string{"tsconfig.json", "jsconfig.json", "package.json"},
	},
	".py": {
		Language:    "python",
		Binary:      "pyright-langserver",
		Args:        []string{"--stdio"},
		RootMarkers: []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile"},
	},
	".pyi": {
		Language:    "python",
		Binary:      "pyright-langserver",
		Args:        []string{"--stdio"},
		RootMarkers: []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile"},
	},
}

// ForExtension returns the default server config for a file extension
// (e.g. ".go"). The second return reports whether a server is registered.
func ForExtension(ext string) (ServerConfig, bool) {
	cfg, ok := defaultServers[ext]
	return cfg, ok
}
