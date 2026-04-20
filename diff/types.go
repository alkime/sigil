package diff

import "time"

type Session struct {
	ID         string     `yaml:"id"`
	Repo       string     `yaml:"repo"`
	PRNumber   int        `yaml:"pr_number"`
	PRTitle    string     `yaml:"pr_title"`
	BaseBranch string     `yaml:"base_branch"`
	BaseSHA    string     `yaml:"base_sha"`
	HeadSHA    string     `yaml:"head_sha"`
	Branch     string     `yaml:"branch"`
	CreatedAt  time.Time  `yaml:"created_at"`
	UpdatedAt  time.Time  `yaml:"updated_at"`
	Snapshots  []Snapshot `yaml:"snapshots"`
}

type Snapshot struct {
	Base       string    `yaml:"base"`
	Head       string    `yaml:"head"`
	ObservedAt time.Time `yaml:"observed_at"`
}

type CommentContext struct {
	Before []string `yaml:"before"`
	Target string   `yaml:"target"`
	After  []string `yaml:"after"`
}

type Comment struct {
	ID          string         `yaml:"id"`
	File        string         `yaml:"file"`
	HunkHeader  string         `yaml:"hunk_header"`
	LineHint    int            `yaml:"line_hint"`
	Side        string         `yaml:"side"`
	Context     CommentContext `yaml:"context"`
	Body        string         `yaml:"body"`
	Author      string         `yaml:"author"`
	CreatedAt   time.Time      `yaml:"created_at"`
	UpdatedAt   time.Time      `yaml:"updated_at"`
	Tags        []string       `yaml:"tags"`
	Resolved    bool           `yaml:"resolved"`
	Orphaned    bool           `yaml:"orphaned"`
	SnapshotRef string         `yaml:"snapshot_ref"`
}

type SessionsIndexEntry struct {
	PRNumber  int       `yaml:"pr_number"`
	Path      string    `yaml:"path"`
	UpdatedAt time.Time `yaml:"updated_at"`
}
