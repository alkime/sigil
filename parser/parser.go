package parser

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alkime/sigil/model"
	"gopkg.in/yaml.v3"
)

var (
	refMarkerRe      = regexp.MustCompile(`^\s*<!--\s*@review-ref\s+(\d{4})\s*-->\s*$`)
	backmatterStartRe = regexp.MustCompile(`^\s*<!--\s*$`)
	backmatterTagRe   = regexp.MustCompile(`^\s*@review-backmatter\s*$`)
)

// Parse reads a Markdown file and returns a parsed Document.
func Parse(filePath string) (*model.Document, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return ParseContent(filePath, data)
}

// ParseContent parses Markdown content into a Document without file I/O.
func ParseContent(filePath string, content []byte) (*model.Document, error) {
	rawLines := strings.Split(string(content), "\n")
	// Remove trailing empty line from final newline
	if len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	doc := &model.Document{
		FilePath:              filePath,
		RawLines:              rawLines,
		CommentByID:           make(map[string]*model.ReviewComment),
		CommentedContentLines: make(map[int][]string),
	}

	// Find backmatter block
	backmatterStart, backmatterEnd := findBackmatter(rawLines)
	backmatterLines := map[int]bool{}
	if backmatterStart >= 0 {
		for i := backmatterStart; i <= backmatterEnd; i++ {
			backmatterLines[i] = true
		}
		if err := parseBackmatter(rawLines[backmatterStart:backmatterEnd+1], doc); err != nil {
			return nil, fmt.Errorf("parsing backmatter: %w", err)
		}
	}

	// Find ref markers
	refMarkerLines := map[int]bool{}
	for i, line := range rawLines {
		if m := refMarkerRe.FindStringSubmatch(line); m != nil {
			doc.RefMarkers = append(doc.RefMarkers, model.RefMarker{
				ID:         m[1],
				SourceLine: i,
			})
			refMarkerLines[i] = true
		}
	}

	// Build ContentLines by filtering out ref markers and backmatter
	for i, line := range rawLines {
		if refMarkerLines[i] || backmatterLines[i] {
			continue
		}
		doc.ContentToSource = append(doc.ContentToSource, i)
		doc.ContentLines = append(doc.ContentLines, line)
	}

	// Build CommentedContentLines: for each comment, find its ref marker,
	// then mark the content lines that fall within the comment's span.
	buildCommentedLines(doc)

	return doc, nil
}

// findBackmatter scans from the end of the file looking for the backmatter block.
// Returns (startLine, endLine) or (-1, -1) if not found.
func findBackmatter(lines []string) (int, int) {
	// The backmatter block ends with a line containing just "-->"
	// and starts with "<!--" followed by "@review-backmatter" on the next line,
	// OR starts with "<!--" on the same line containing @review-backmatter.
	endLine := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "-->" {
			endLine = i
			break
		}
		// Skip trailing blank lines
		if trimmed != "" {
			break
		}
	}
	if endLine < 0 {
		return -1, -1
	}

	// Now scan up from endLine to find the opening
	for i := endLine - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		// Check for single-line start: "<!-- @review-backmatter" or just "<!--"
		if trimmed == "@review-backmatter" {
			// Previous line should be "<!--"
			if i > 0 && backmatterStartRe.MatchString(lines[i-1]) {
				return i - 1, endLine
			}
			continue
		}
		// Check for combined start: "<!-- @review-backmatter"
		if strings.HasPrefix(trimmed, "<!--") && strings.Contains(trimmed, "@review-backmatter") {
			return i, endLine
		}
	}

	return -1, -1
}

// parseBackmatter extracts the YAML content from the backmatter block and populates doc.Comments.
func parseBackmatter(block []string, doc *model.Document) error {
	// Strip the HTML comment delimiters and the @review-backmatter tag
	var yamlLines []string
	inYAML := false
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if trimmed == "-->" {
			break
		}
		if strings.Contains(trimmed, "@review-backmatter") {
			inYAML = true
			continue
		}
		if !inYAML {
			// Skip "<!--" line
			continue
		}
		yamlLines = append(yamlLines, line)
	}

	yamlContent := strings.Join(yamlLines, "\n")
	if strings.TrimSpace(yamlContent) == "" {
		return nil
	}

	// Parse as map[string]ReviewComment (keys are quoted IDs like "0001")
	// Also handle unquoted integer keys by zero-padding them.
	var raw yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	if raw.Kind != yaml.DocumentNode || len(raw.Content) == 0 {
		return nil
	}

	mapNode := raw.Content[0]
	if mapNode.Kind != yaml.MappingNode {
		return fmt.Errorf("expected YAML map, got %v", mapNode.Kind)
	}

	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		keyNode := mapNode.Content[i]
		valNode := mapNode.Content[i+1]

		id := normalizeID(keyNode)
		if id == "" {
			continue
		}

		var rc model.ReviewComment
		if err := valNode.Decode(&rc); err != nil {
			return fmt.Errorf("decoding comment %s: %w", id, err)
		}
		rc.ID = id
		doc.Comments = append(doc.Comments, rc)
		doc.CommentByID[id] = &doc.Comments[len(doc.Comments)-1]
	}

	return nil
}

// normalizeID converts a YAML key node to a zero-padded 4-digit string ID.
func normalizeID(node *yaml.Node) string {
	val := node.Value
	if val == "" {
		return ""
	}
	// If it's an integer (YAML parsed bare 0001 as 1), zero-pad it
	if n, err := strconv.Atoi(val); err == nil {
		return fmt.Sprintf("%04d", n)
	}
	// Already a string like "0001"
	return val
}

// buildCommentedLines maps content line indices to comment IDs based on
// ref markers and comment offset/span.
func buildCommentedLines(doc *model.Document) {
	// Build an inverted map: source line -> content line index
	sourceToContent := make(map[int]int, len(doc.ContentToSource))
	for ci, si := range doc.ContentToSource {
		sourceToContent[si] = ci
	}

	for _, marker := range doc.RefMarkers {
		comment, ok := doc.CommentByID[marker.ID]
		if !ok {
			continue
		}

		// The comment covers source lines starting at marker.SourceLine + offset
		// for `span` lines. But these are source lines (in RawLines), and we need
		// to find which content lines they map to.
		startSource := marker.SourceLine + comment.Offset
		for s := startSource; s < startSource+comment.Span; s++ {
			if ci, ok := sourceToContent[s]; ok {
				doc.CommentedContentLines[ci] = append(doc.CommentedContentLines[ci], marker.ID)
			}
		}
	}

	// Sort comment IDs on each line for determinism
	for _, ids := range doc.CommentedContentLines {
		sort.Strings(ids)
	}
}
