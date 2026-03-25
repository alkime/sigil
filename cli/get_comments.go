package cli

import (
	"fmt"

	"github.com/alkime/sigil/model"
	"github.com/alkime/sigil/parser"
)

// Run parses the file and prints matching comments as plain-text blocks.
func (c *GetCommentsCmd) Run(ctx *CLIContext) error {
	doc, err := parser.Parse(c.File)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// Build a map from comment ID to its ref marker for source-line lookup.
	markerByID := make(map[string]*model.RefMarker, len(doc.RefMarkers))
	for i := range doc.RefMarkers {
		markerByID[doc.RefMarkers[i].ID] = &doc.RefMarkers[i]
	}

	for _, comment := range doc.Comments {
		if c.Open && comment.Status != "open" {
			continue
		}
		if c.Resolved && comment.Status != "resolved" {
			continue
		}

		marker := markerByID[comment.ID]

		fmt.Fprintf(ctx.Out, "=== Comment %s [%s] ===\n", comment.ID, comment.Status)

		if marker != nil {
			startSource := marker.SourceLine + comment.Offset
			endSource := startSource + comment.Span
			// Display 1-based line numbers.
			fmt.Fprintf(ctx.Out, "Lines: %d-%d\n", startSource+1, endSource)
		}

		fmt.Fprintf(ctx.Out, "---\n")
		fmt.Fprintf(ctx.Out, "%s\n", comment.Comment)
		fmt.Fprintf(ctx.Out, "---\n")

		// Print covered source text.
		if marker != nil {
			startSource := marker.SourceLine + comment.Offset
			for i := 0; i < comment.Span; i++ {
				lineIdx := startSource + i
				if lineIdx >= 0 && lineIdx < len(doc.RawLines) {
					fmt.Fprintf(ctx.Out, "> %s\n", doc.RawLines[lineIdx])
				}
			}
		}

		fmt.Fprintln(ctx.Out)
	}

	return nil
}
