package diff

import (
	"os"
	"path/filepath"
	"testing"
)

func setupViewedTmpHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	return dir
}

func TestLoadViewedState_MissingFile(t *testing.T) {
	setupViewedTmpHome(t)
	got, err := LoadViewedState("acme", "repo", PRSessionKey(7), "base", "head")
	if err != nil {
		t.Fatalf("expected no error on missing file, got %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil state")
	}
	if got.Len() != 0 {
		t.Errorf("expected empty state, got Len=%d", got.Len())
	}
}

func TestLoadViewedState_Malformed(t *testing.T) {
	setupViewedTmpHome(t)
	dir := SnapshotDir("acme", "repo", PRSessionKey(7), "base", "head")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ViewedStateFileName)
	if err := os.WriteFile(path, []byte("viewed: [unterminated"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadViewedState("acme", "repo", PRSessionKey(7), "base", "head")
	if err == nil {
		t.Fatalf("expected error on malformed yaml, got nil")
	}
	if got == nil || got.Len() != 0 {
		t.Errorf("expected empty state on error, got %v / Len=%d", got, got.Len())
	}
}

func TestViewedState_MarkUnmarkToggle(t *testing.T) {
	s := NewViewedState()

	if s.IsViewed("a.go") {
		t.Error("fresh state should report unviewed")
	}
	s.Mark("a.go")
	if !s.IsViewed("a.go") {
		t.Error("Mark did not take effect")
	}
	if s.Len() != 1 {
		t.Errorf("Len = %d, want 1", s.Len())
	}

	// Mark twice is idempotent.
	s.Mark("a.go")
	if s.Len() != 1 {
		t.Errorf("Len after double Mark = %d, want 1", s.Len())
	}

	s.Unmark("a.go")
	if s.IsViewed("a.go") {
		t.Error("Unmark did not take effect")
	}

	// Toggle: off -> on -> off.
	if now := s.Toggle("b.go"); !now {
		t.Error("Toggle off->on should return true")
	}
	if !s.IsViewed("b.go") {
		t.Error("expected b.go viewed after Toggle")
	}
	if now := s.Toggle("b.go"); now {
		t.Error("Toggle on->off should return false")
	}
	if s.IsViewed("b.go") {
		t.Error("expected b.go unviewed after second Toggle")
	}
}

func TestViewedState_NilSafe(t *testing.T) {
	var s *ViewedState
	if s.IsViewed("a.go") {
		t.Error("nil state should report unviewed")
	}
	if s.Len() != 0 {
		t.Error("nil state Len should be 0")
	}
	// These should not panic on nil.
	s.Mark("a.go")
	s.Unmark("a.go")
	_ = s.Toggle("a.go")
}

func TestViewedState_SaveLoadRoundtrip(t *testing.T) {
	setupViewedTmpHome(t)
	s := NewViewedState()
	s.Mark("pkg/z.go")
	s.Mark("pkg/a.go")
	s.Mark("pkg/m.go")

	if err := s.Save("acme", "repo", PRSessionKey(7), "base", "head"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// On-disk file should be sorted alphabetically.
	data, err := os.ReadFile(filepath.Join(SnapshotDir("acme", "repo", PRSessionKey(7), "base", "head"), ViewedStateFileName))
	if err != nil {
		t.Fatal(err)
	}
	want := "viewed:\n    - pkg/a.go\n    - pkg/m.go\n    - pkg/z.go\n"
	if string(data) != want {
		t.Errorf("on-disk content mismatch\ngot:\n%s\nwant:\n%s", data, want)
	}

	loaded, err := LoadViewedState("acme", "repo", PRSessionKey(7), "base", "head")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Len() != 3 {
		t.Errorf("loaded Len = %d, want 3", loaded.Len())
	}
	for _, p := range []string{"pkg/a.go", "pkg/m.go", "pkg/z.go"} {
		if !loaded.IsViewed(p) {
			t.Errorf("expected %q viewed after reload", p)
		}
	}
}
