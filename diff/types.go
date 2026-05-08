package diff

import "time"

// SessionKind distinguishes a GitHub PR session from a local-only session
// (created via `sigil diff --local` / `--base <ref>`).
type SessionKind string

const (
	SessionKindPR    SessionKind = "pr"
	SessionKindLocal SessionKind = "local"
)

type Session struct {
	ID         string      `yaml:"id"`
	Repo       string      `yaml:"repo"`
	Kind       SessionKind `yaml:"kind,omitempty"`
	PRNumber   int         `yaml:"pr_number"`
	PRTitle    string      `yaml:"pr_title"`
	BaseBranch string      `yaml:"base_branch"`
	BaseSHA    string      `yaml:"base_sha"`
	HeadSHA    string      `yaml:"head_sha"`
	Branch     string      `yaml:"branch"`
	CreatedAt  time.Time   `yaml:"created_at"`
	UpdatedAt  time.Time   `yaml:"updated_at"`
	Snapshots  []Snapshot  `yaml:"snapshots"`
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

// SessionsIndexEntry is one row of {base}/{org}/{repo}/sessions.yaml.
// Key is the on-disk session key (filled for both PR and local sessions).
// PRNumber is retained so legacy index entries (written before Key existed)
// continue to deserialize cleanly; for local sessions it stays zero.
type SessionsIndexEntry struct {
	Key       string    `yaml:"key,omitempty"`
	PRNumber  int       `yaml:"pr_number,omitempty"`
	Path      string    `yaml:"path"`
	UpdatedAt time.Time `yaml:"updated_at"`
}

// SessionKey returns the on-disk session key for an index entry, falling back
// to PRNumber for legacy rows that predate the Key field.
func (e SessionsIndexEntry) SessionKey() SessionKey {
	if e.Key != "" {
		return SessionKey(e.Key)
	}
	return PRSessionKey(e.PRNumber)
}
