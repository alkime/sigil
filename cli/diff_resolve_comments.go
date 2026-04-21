package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/alkime/sigil/diff"
)

// DiffResolveCmd marks diff comments as resolved.
type DiffResolveCmd struct {
	IDs    []string `arg:"" help:"Comment IDs to resolve."`
	Author string   `help:"Override author (default: git config user.name or $USER)."`
}

// Run flips resolved=true for each given comment ID and saves.
func (c *DiffResolveCmd) Run(ctx *CLIContext) error {
	return setDiffCommentResolved(ctx, c.IDs, true)
}

// DiffUnresolveCmd marks diff comments as unresolved.
type DiffUnresolveCmd struct {
	IDs    []string `arg:"" help:"Comment IDs to unresolve."`
	Author string   `help:"Override author (default: git config user.name or $USER)."`
}

// Run flips resolved=false for each given comment ID and saves.
func (c *DiffUnresolveCmd) Run(ctx *CLIContext) error {
	return setDiffCommentResolved(ctx, c.IDs, false)
}

func setDiffCommentResolved(ctx *CLIContext, ids []string, resolved bool) error {
	session, _, _, err := diff.Resolve(context.Background(), diff.ResolveOpts{
		SessionID:    ctx.DiffSession,
		IncludeDraft: ctx.DiffDraft,
	})
	if err != nil {
		return fmt.Errorf("resolve session: %w", err)
	}

	org, repo := splitRepo(session.Repo)
	comments, err := diff.LoadComments(org, repo, session.PRNumber)
	if err != nil {
		return fmt.Errorf("load comments: %w", err)
	}

	for _, id := range ids {
		found := false
		for i := range comments {
			if comments[i].ID == id {
				comments[i].Resolved = resolved
				comments[i].UpdatedAt = time.Now().UTC()
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("comment %s not found", id)
		}
	}

	if err := diff.SaveComments(org, repo, session.PRNumber, comments); err != nil {
		return fmt.Errorf("save comments: %w", err)
	}

	for _, id := range ids {
		if resolved {
			fmt.Fprintf(ctx.Out, "Resolved %s\n", id)
		} else {
			fmt.Fprintf(ctx.Out, "Unresolved %s\n", id)
		}
	}
	return nil
}
