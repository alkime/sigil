package diff

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ReviewOrderFileName is the path (relative to the worktree root) that sigil
// reads at startup to decide file ordering and blurbs.
const ReviewOrderFileName = ".sigil/review-order.yaml"

// ReviewOrderEntry is a single line in the review-order file: a path (matched
// against ParsedFile.NewPath, then OldPath for deletes) and an optional note.
type ReviewOrderEntry struct {
	Path string `yaml:"path"`
	Note string `yaml:"note,omitempty"`
}

// ReviewOrder is the parsed .sigil/review-order.yaml. Entries are preserved
// in the order they appear in the file.
type ReviewOrder struct {
	Entries []ReviewOrderEntry `yaml:"files"`
}

// LoadReviewOrder reads the review-order file from the given worktree.
// Returns (nil, nil) when the file does not exist (absent ≠ error).
// Returns (nil, err) when the file exists but can't be parsed.
func LoadReviewOrder(worktreePath string) (*ReviewOrder, error) {
	if worktreePath == "" {
		return nil, nil
	}
	path := filepath.Join(worktreePath, ReviewOrderFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", ReviewOrderFileName, err)
	}
	var r ReviewOrder
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ReviewOrderFileName, err)
	}
	return &r, nil
}

// Reorder returns files sorted per r.Entries (matched entries first, in list
// order), followed by any PR files not named in r.Entries (in their original
// diff order). The second return is the number of entries in r that did not
// match any file in the PR.
func (r *ReviewOrder) Reorder(files []ParsedFile) ([]ParsedFile, int) {
	if r == nil || len(r.Entries) == 0 {
		return files, 0
	}

	byPath := make(map[string]int, len(files)*2)
	for i, f := range files {
		if f.NewPath != "" {
			byPath[f.NewPath] = i
		}
		if f.OldPath != "" && f.OldPath != f.NewPath {
			byPath[f.OldPath] = i
		}
	}

	used := make([]bool, len(files))
	out := make([]ParsedFile, 0, len(files))
	dropped := 0
	for _, e := range r.Entries {
		idx, ok := byPath[e.Path]
		if !ok || used[idx] {
			dropped++
			continue
		}
		out = append(out, files[idx])
		used[idx] = true
	}
	for i, f := range files {
		if !used[i] {
			out = append(out, f)
		}
	}
	return out, dropped
}

// NoteFor returns the note (if any) for the given file. Matches on NewPath
// first, then OldPath. Returns "" when no match.
func (r *ReviewOrder) NoteFor(f ParsedFile) string {
	if r == nil {
		return ""
	}
	for _, e := range r.Entries {
		if e.Path == f.NewPath || (f.OldPath != "" && e.Path == f.OldPath) {
			return e.Note
		}
	}
	return ""
}
