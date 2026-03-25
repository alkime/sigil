package cli

import (
	"fmt"

	"github.com/alkime/sigil/parser"
	"github.com/alkime/sigil/writer"
)

// Run marks the specified comments as resolved.
func (c *ResolveCommentsCmd) Run(ctx *CLIContext) error {
	return setCommentStatus(c.File, c.IDs, "resolved", ctx)
}

// Run marks the specified comments as unresolved (open).
func (c *UnresolveCommentsCmd) Run(ctx *CLIContext) error {
	return setCommentStatus(c.File, c.IDs, "open", ctx)
}

// setCommentStatus is a shared helper that sets the status of the given
// comment IDs and writes the file back to disk.
func setCommentStatus(file string, ids []string, status string, ctx *CLIContext) error {
	doc, err := parser.Parse(file)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	for _, rawID := range ids {
		id := NormalizeID(rawID)
		existing := doc.CommentByID[id]
		if existing == nil {
			return fmt.Errorf("comment %s not found", id)
		}

		doc, err = writer.UpdateComment(doc, id, existing.Comment, status)
		if err != nil {
			return fmt.Errorf("updating comment %s: %w", id, err)
		}

		// Re-lookup after re-parse (UpdateComment returns a fresh *Document).
		printStatus(ctx, id, status)
	}

	return nil
}

func printStatus(ctx *CLIContext, id string, status string) {
	switch status {
	case "resolved":
		fmt.Fprintf(ctx.Out, "Resolved %s\n", id)
	case "open":
		fmt.Fprintf(ctx.Out, "Unresolved %s\n", id)
	default:
		fmt.Fprintf(ctx.Out, "Updated %s -> %s\n", id, status)
	}
}

