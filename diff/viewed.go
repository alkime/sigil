package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ViewedStateFileName is the name of the per-snapshot file that records which
// files the reviewer has marked "viewed" (they're done looking at it for this
// snapshot and don't want Tab/S-Tab to stop on it anymore).
const ViewedStateFileName = "viewed.yaml"

// ViewedState tracks the set of file paths the reviewer has marked viewed.
// Scoped per-snapshot so a new PR snapshot starts with a clean slate — this
// means force-pushes and new commits don't silently carry forward stale
// viewed marks onto files that may have changed.
type ViewedState struct {
	paths map[string]struct{}
}

// viewedDoc is the on-disk YAML shape. The wrapper object leaves room for
// future fields without a breaking change.
type viewedDoc struct {
	Viewed []string `yaml:"viewed"`
}

// NewViewedState returns an empty state.
func NewViewedState() *ViewedState {
	return &ViewedState{paths: map[string]struct{}{}}
}

// LoadViewedState reads the viewed-state file for a snapshot. Always returns
// a non-nil *ViewedState — an empty state when the file doesn't exist.
// Returns an error only when the file exists but can't be parsed.
func LoadViewedState(org, repo string, key SessionKey, baseSHA, headSHA string) (*ViewedState, error) {
	path := viewedStatePath(org, repo, key, baseSHA, headSHA)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewViewedState(), nil
		}
		return NewViewedState(), fmt.Errorf("read %s: %w", ViewedStateFileName, err)
	}
	var doc viewedDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return NewViewedState(), fmt.Errorf("parse %s: %w", ViewedStateFileName, err)
	}
	s := NewViewedState()
	for _, p := range doc.Viewed {
		s.paths[p] = struct{}{}
	}
	return s, nil
}

// Save writes the state to the snapshot dir, creating the dir if necessary.
// Paths are sorted for stable on-disk content.
func (v *ViewedState) Save(org, repo string, key SessionKey, baseSHA, headSHA string) error {
	dir := SnapshotDir(org, repo, key, baseSHA, headSHA)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	doc := viewedDoc{Viewed: v.sortedPaths()}
	return writeYAML(filepath.Join(dir, ViewedStateFileName), doc)
}

// Mark records path as viewed.
func (v *ViewedState) Mark(path string) {
	if v == nil || path == "" {
		return
	}
	v.paths[path] = struct{}{}
}

// Unmark clears the viewed mark on path.
func (v *ViewedState) Unmark(path string) {
	if v == nil {
		return
	}
	delete(v.paths, path)
}

// Toggle flips the viewed mark on path and returns the new state (true = now
// viewed, false = now unviewed).
func (v *ViewedState) Toggle(path string) bool {
	if v == nil || path == "" {
		return false
	}
	if _, ok := v.paths[path]; ok {
		delete(v.paths, path)
		return false
	}
	v.paths[path] = struct{}{}
	return true
}

// IsViewed reports whether path is currently marked viewed.
func (v *ViewedState) IsViewed(path string) bool {
	if v == nil {
		return false
	}
	_, ok := v.paths[path]
	return ok
}

// Len returns the count of viewed files.
func (v *ViewedState) Len() int {
	if v == nil {
		return 0
	}
	return len(v.paths)
}

func (v *ViewedState) sortedPaths() []string {
	out := make([]string, 0, len(v.paths))
	for p := range v.paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func viewedStatePath(org, repo string, key SessionKey, baseSHA, headSHA string) string {
	return filepath.Join(SnapshotDir(org, repo, key, baseSHA, headSHA), ViewedStateFileName)
}
