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
	lines = appendBackmatter(lines, allComments)

	// Write to file
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(doc.FilePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing file: %w", err)
	}

	// Re-parse to get a fresh document
	return parser.Parse(doc.FilePath)
}

// UpdateComment updates an existing comment's text and/or status and writes back to disk.
func UpdateComment(doc *model.Document, id string, newText string, newStatus string) (*model.Document, error) {
	// Update the comment in the existing list
	comments := slices.Clone(doc.Comments)
	for i := range comments {
		if comments[i].ID == id {
			comments[i].Comment = newText
			comments[i].Status = newStatus
			break
		}
	}

	return rewriteBackmatter(doc, comments)
}

// DeleteComment removes a comment's ref marker and backmatter entry, then writes back to disk.
func DeleteComment(doc *model.Document, id string) (*model.Document, error) {
	lines := slices.Clone(doc.RawLines)

	// Remove the ref marker line for this comment
	refMarkerRe := fmt.Sprintf("<!-- @review-ref %s -->", id)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == refMarkerRe {
			lines = slices.Delete(lines, i, i+1)
			break
		}
	}

	// Remove existing backmatter
	bmStart, bmEnd := findBackmatter(lines)
	if bmStart >= 0 {
		lines = slices.Delete(lines, bmStart, bmEnd+1)
	}

	// Rebuild backmatter without the deleted comment
	var remaining []model.ReviewComment
	for _, c := range doc.Comments {
		if c.ID != id {
			remaining = append(remaining, c)
		}
	}

	lines = appendBackmatter(lines, remaining)

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(doc.FilePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing file: %w", err)
	}

	return parser.Parse(doc.FilePath)
}

// rewriteBackmatter replaces just the backmatter with updated comments.
func rewriteBackmatter(doc *model.Document, comments []model.ReviewComment) (*model.Document, error) {
	lines := slices.Clone(doc.RawLines)

	bmStart, bmEnd := findBackmatter(lines)
	if bmStart >= 0 {
		lines = slices.Delete(lines, bmStart, bmEnd+1)
	}

	lines = appendBackmatter(lines, comments)

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(doc.FilePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing file: %w", err)
	}

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

// appendBackmatter trims trailing blank lines from lines, then appends one blank separator and the backmatter.
func appendBackmatter(lines []string, comments []model.ReviewComment) []string {
	// Trim trailing blank lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(comments) == 0 {
		return lines
	}
	lines = append(lines, "")
	lines = append(lines, buildBackmatter(comments)...)
	return lines
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
