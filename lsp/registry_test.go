package lsp

import "testing"

func TestForExtension_Go(t *testing.T) {
	cfg, ok := ForExtension(".go")
	if !ok {
		t.Fatal("ForExtension(.go): ok=false, want true")
	}
	if cfg.Language != "go" {
		t.Errorf("Language = %q, want %q", cfg.Language, "go")
	}
	if cfg.Binary != "gopls" {
		t.Errorf("Binary = %q, want %q", cfg.Binary, "gopls")
	}
	foundGoMod := false
	for _, m := range cfg.RootMarkers {
		if m == "go.mod" {
			foundGoMod = true
			break
		}
	}
	if !foundGoMod {
		t.Errorf("RootMarkers = %v, want one to be %q", cfg.RootMarkers, "go.mod")
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
