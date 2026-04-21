package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/alkime/sigil/diff"
)

// Run creates a tea.Program and runs the diff TUI for the given session and diff.
// worktreePath is the absolute path to the matching worktree (empty when unknown).
func Run(session *diff.Session, pd *diff.ParsedDiff, worktreePath string) error {
	m := New(session, pd, worktreePath)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

// RunWithResolve resolves a session from opts and runs the TUI.
// If multiple PRs are found, launches the picker first.
func RunWithResolve(ctx context.Context, opts diff.ResolveOpts) error {
	session, pd, workspaceDir, err := diff.Resolve(ctx, opts)
	if err != nil {
		var pickerNeeded *diff.ErrPickerNeeded
		if errors.As(err, &pickerNeeded) {
			chosen, pickErr := RunPicker(pickerNeeded.Candidates)
			if pickErr != nil {
				return fmt.Errorf("picker: %w", pickErr)
			}
			opts.SessionID = chosen.WorktreePath // use worktree to re-detect
			// Re-resolve with a specific session if we can identify it — for now
			// just set a marker and retry (the session won't exist yet; let Resolve create it).
			// Use the branch from the chosen candidate as a hint.
			opts2 := diff.ResolveOpts{
				IncludeDraft: opts.IncludeDraft,
				CWD:          chosen.WorktreePath,
			}
			session, pd, workspaceDir, err = diff.Resolve(ctx, opts2)
			if err != nil {
				return fmt.Errorf("resolve after pick: %w", err)
			}
		} else {
			return err
		}
	}
	return Run(session, pd, workspaceDir)
}
