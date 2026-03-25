package cli

import (
	"fmt"

	"github.com/alkime/sigil/parser"
	"github.com/alkime/sigil/writer"
)

// Run appends a reply to the specified comment and writes the file back.
func (c *ReplyCommentCmd) Run(ctx *CLIContext) error {
	doc, err := parser.Parse(c.File)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	id := NormalizeID(c.ID)
	existing := doc.CommentByID[id]
	if existing == nil {
		return fmt.Errorf("comment %s not found", id)
	}

	newText := existing.Comment + "\n\nREPLY: " + c.ReplyText

	_, err = writer.UpdateComment(doc, id, newText, existing.Status)
	if err != nil {
		return fmt.Errorf("updating comment %s: %w", id, err)
	}

	fmt.Fprintf(ctx.Out, "Replied to %s\n", id)
	return nil
}
