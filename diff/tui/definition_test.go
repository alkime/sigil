package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	dtui "github.com/alkime/sigil/diff/tui"
)

// TestGoToDef_chordRequiresTwoG ensures a single 'g' press is a no-op; chord
// detection only fires on the second 'g'.
func TestGoToDef_chordRequiresTwoG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Land on a real diff line (focusedLine 0 is the hunk header).
	m1 := send(m, "j").(dtui.Model)
	before := m1.StatusMsg()

	// Single 'g' should not trigger anything.
	m2 := send(m1, "g").(dtui.Model)
	if m2.StatusMsg() != before {
		t.Errorf("single g changed status: before=%q after=%q", before, m2.StatusMsg())
	}
}

// TestGoToDef_noWorktree shows a graceful "LSP disabled" message when the
// worktree path isn't known, so we never attempt to spawn an LSP server.
func TestGoToDef_noWorktree(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// Move to "package main" (focusedLine 1, col 0 is on 'p' — a word rune).
	m1 := send(m, "j").(dtui.Model)

	// Fire the 'gg' chord.
	m2 := send(m1, "g", "g").(dtui.Model)

	status := m2.StatusMsg()
	if !strings.Contains(status, "LSP disabled") && !strings.Contains(status, "no LSP configured") {
		t.Errorf("expected LSP disabled / no LSP configured message, got: %q", status)
	}
}

// TestGoToDef_hunkHeaderNoSymbol ensures gd on a hunk header reports "no symbol".
func TestGoToDef_hunkHeaderNoSymbol(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// focusedLine 0 is the hunk header.
	m2 := send(m, "g", "g").(dtui.Model)
	if !strings.Contains(m2.StatusMsg(), "no symbol under cursor") {
		t.Errorf("expected 'no symbol under cursor', got: %q", m2.StatusMsg())
	}
}

// TestJumpBack_emptyHistory shows a graceful message rather than panicking.
func TestJumpBack_emptyHistory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := setupModel(t)

	// ctrl+o with no history.
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if !strings.Contains(m2.(dtui.Model).StatusMsg(), "no previous location") {
		t.Errorf("expected 'no previous location', got: %q", m2.(dtui.Model).StatusMsg())
	}
}
