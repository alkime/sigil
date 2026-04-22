package lsp

import "testing"

func TestForExtension(t *testing.T) {
	cases := []struct {
		ext         string
		wantLang    string
		wantBinary  string
		wantMarker  string
	}{
		{".go", "go", "gopls", "go.mod"},
		{".ts", "typescript", "typescript-language-server", "tsconfig.json"},
		{".tsx", "typescriptreact", "typescript-language-server", "tsconfig.json"},
		{".py", "python", "pyright-langserver", "pyproject.toml"},
		{".pyi", "python", "pyright-langserver", "pyproject.toml"},
	}
	for _, tc := range cases {
		t.Run(tc.ext, func(t *testing.T) {
			cfg, ok := ForExtension(tc.ext)
			if !ok {
				t.Fatalf("ForExtension(%q): ok=false, want true", tc.ext)
			}
			if cfg.Language != tc.wantLang {
				t.Errorf("Language = %q, want %q", cfg.Language, tc.wantLang)
			}
			if cfg.Binary != tc.wantBinary {
				t.Errorf("Binary = %q, want %q", cfg.Binary, tc.wantBinary)
			}
			found := false
			for _, m := range cfg.RootMarkers {
				if m == tc.wantMarker {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("RootMarkers = %v, want one to be %q", cfg.RootMarkers, tc.wantMarker)
			}
		})
	}
}

func TestForExtension_Unknown(t *testing.T) {
	if _, ok := ForExtension(".xyz"); ok {
		t.Error("ForExtension(.xyz): ok=true, want false")
	}
	if _, ok := ForExtension(""); ok {
		t.Error("ForExtension(\"\"): ok=true, want false")
	}
}
