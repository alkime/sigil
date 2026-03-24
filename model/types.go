package model

// ReviewComment represents a single review comment parsed from backmatter.
type ReviewComment struct {
	ID      string `yaml:"-"`
	Offset  int    `yaml:"offset"`
	Span    int    `yaml:"span"`
	Comment string `yaml:"comment"`
	Status  string `yaml:"status"`
}

// RefMarker represents an inline ref marker found in the source.
type RefMarker struct {
	ID         string
	SourceLine int // 0-based line number in RawLines where the marker appears
}

// Document is the fully parsed representation of a Markdown file with review data.
type Document struct {
	FilePath string
	RawLines []string // Original file lines (including markers and backmatter)

	// ContentLines are RawLines with ref markers and backmatter stripped.
	// This is what gets rendered by Glamour.
	ContentLines []string

	// ContentToSource maps each ContentLines index to its 0-based index in RawLines.
	ContentToSource []int

	RefMarkers  []RefMarker
	Comments    []ReviewComment
	CommentByID map[string]*ReviewComment

	// CommentedContentLines maps a ContentLines index to the comment IDs
	// that cover that line. Used for gutter rendering and navigation.
	CommentedContentLines map[int][]string
}
