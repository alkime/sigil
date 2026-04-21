package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alkime/sigil/diff"
)

// DiffGetCommentsCmd prints comments for the current diff session.
type DiffGetCommentsCmd struct {
	Open     bool `help:"Show only open comments." xor:"status"`
	Resolved bool `help:"Show only resolved comments." xor:"status"`
}

// Run resolves the session and prints matching comments as plain-text blocks.
func (c *DiffGetCommentsCmd) Run(ctx *CLIContext) error {
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

	for _, comment := range comments {
		if c.Open && comment.Resolved {
			continue
		}
		if c.Resolved && !comment.Resolved {
			continue
		}
		printDiffComment(ctx, comment)
	}

	return nil
}

func printDiffComment(ctx *CLIContext, c diff.Comment) {
	status := "open"
	if c.Resolved {
		status = "resolved"
	}

	fmt.Fprintf(ctx.Out, "=== Comment %s [%s] ===\n", c.ID, status)
	fmt.Fprintf(ctx.Out, "File: %s\n", c.File)
	fmt.Fprintf(ctx.Out, "Hunk target: `%s`\n", c.Context.Target)
	fmt.Fprintf(ctx.Out, "---\n")
	fmt.Fprintf(ctx.Out, "%s\n", c.Body)
	fmt.Fprintf(ctx.Out, "---\n")

	if c.HunkHeader != "" {
		fmt.Fprintf(ctx.Out, "> %s\n", c.HunkHeader)
	}
	for _, line := range c.Context.Before {
		fmt.Fprintf(ctx.Out, "> %s\n", line)
	}
	fmt.Fprintf(ctx.Out, "> %s\n", c.Context.Target)
	for _, line := range c.Context.After {
		fmt.Fprintf(ctx.Out, "> %s\n", line)
	}

	fmt.Fprintln(ctx.Out)
}

func splitRepo(repo string) (org, name string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", repo
}
