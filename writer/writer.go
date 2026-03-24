package writer

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/alkime/sigil/model"
	"github.com/alkime/sigil/parser"
)

// WriteComment adds a new review comment to the document and writes it back to disk.
// sourceLine is the 0-based index in RawLines where the ref marker should be inserted
// (the marker goes before the content being commented on).
// span is the number of source lines the comment covers.
// Returns the re-parsed document so the TUI can refresh.
func WriteComment(doc *model.Document, sourceLine int, span int, commentText string) (*model.Document, error) {
	nextID := nextCommentID(doc)

	// Build new file lines from RawLines
	lines := slices.Clone(doc.RawLines)

	// Find and remove existing backmatter
	bmStart, bmEnd := findBackmatter(lines)
	if bmStart >= 0 {
		lines = slices.Delete(lines, bmStart, bmEnd+1)
	}

	// Insert ref marker at sourceLine (before the content)
	marker := fmt.Sprintf("<!-- @review-ref %s -->", nextID)
	lines = slices.Insert(lines, sourceLine, marker)

	// Build new comment
	newComment := model.ReviewComment{
		ID:      nextID,
		Offset:  1,
		Span:    span,
		Comment: commentText,
		Status:  "open",
	}

	// Collect all comments (existing + new)
	allComments := append(slices.Clone(doc.Comments), newComment)

	// Append backmatter
	lines = append(lines, "")
	lines = append(lines, buildBackmatter(allComments)...)

	// Write to file
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(doc.FilePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing file: %w", err)
	}

	// Re-parse to get a fresh document
	return parser.Parse(doc.FilePath)
}

// nextCommentID returns the next available zero-padded 4-digit ID.
func nextCommentID(doc *model.Document) string {
	maxID := 0
	for _, c := range doc.Comments {
		if n, err := strconv.Atoi(c.ID); err == nil && n > maxID {
			maxID = n
		}
	}
	return fmt.Sprintf("%04d", maxID+1)
}

// findBackmatter locates the backmatter block in the given lines.
// Returns (startLine, endLine) or (-1, -1) if not found.
func findBackmatter(lines []string) (int, int) {
	endLine := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "-->" {
			endLine = i
			break
		}
		if trimmed != "" {
			break
		}
	}
	if endLine < 0 {
		return -1, -1
	}

	for i := endLine - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "@review-backmatter" {
			if i > 0 && strings.TrimSpace(lines[i-1]) == "<!--" {
				return i - 1, endLine
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<!--") && strings.Contains(trimmed, "@review-backmatter") {
			return i, endLine
		}
	}

	return -1, -1
}

// buildBackmatter serializes all comments into the backmatter block lines.
func buildBackmatter(comments []model.ReviewComment) []string {
	if len(comments) == 0 {
		return nil
	}

	lines := []string{"<!--", "@review-backmatter", ""}

	for _, c := range comments {
		lines = append(lines,
			fmt.Sprintf("%q:", c.ID),
			fmt.Sprintf("  offset: %d", c.Offset),
			fmt.Sprintf("  span: %d", c.Span),
			fmt.Sprintf("  comment: %q", c.Comment),
			fmt.Sprintf("  status: %s", c.Status),
			"",
		)
	}

	lines = append(lines, "-->")
	return lines
}
