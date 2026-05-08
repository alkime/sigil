package diff

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SessionKey is the on-disk identifier for a session under {base}/{org}/{repo}/.
// PR-backed sessions use the bare PR number (preserving the legacy on-disk
// layout); local sessions use a "local-<branch>-vs-<base>" string so they
// can't collide with any positive integer PR number.
type SessionKey string

// PRSessionKey returns the on-disk key for a GitHub PR session.
func PRSessionKey(prNumber int) SessionKey {
	return SessionKey(strconv.Itoa(prNumber))
}

// LocalSessionKey returns the on-disk key for a local (no-PR) session,
// derived from the branch name and the base ref. Slashes in either ref are
// replaced with underscores so the key is filesystem-safe across platforms.
func LocalSessionKey(branch, base string) SessionKey {
	return SessionKey("local-" + sanitizeRef(branch) + "-vs-" + sanitizeRef(base))
}

func sanitizeRef(ref string) string {
	r := strings.ReplaceAll(ref, "/", "_")
	r = strings.ReplaceAll(r, string(filepath.Separator), "_")
	return r
}

// IsLocal reports whether the key was produced by LocalSessionKey.
func (k SessionKey) IsLocal() bool {
	return strings.HasPrefix(string(k), "local-")
}

func BasePath() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "sigil", "diffs")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "sigil", "diffs")
}

func SessionDir(org, repo string, key SessionKey) string {
	return filepath.Join(BasePath(), org, repo, string(key))
}

func SnapshotDir(org, repo string, key SessionKey, baseSHA, headSHA string) string {
	return filepath.Join(SessionDir(org, repo, key), "snapshots", baseSHA+"_"+headSHA)
}

func LoadSession(org, repo string, key SessionKey) (*Session, error) {
	path := filepath.Join(SessionDir(org, repo, key), "session.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func SaveSession(org, repo string, key SessionKey, s *Session) error {
	dir := SessionDir(org, repo, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeYAML(filepath.Join(dir, "session.yaml"), s)
}

func LoadComments(org, repo string, key SessionKey) ([]Comment, error) {
	path := filepath.Join(SessionDir(org, repo, key), "comments.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var comments []Comment
	if err := yaml.Unmarshal(data, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func SaveComments(org, repo string, key SessionKey, comments []Comment) error {
	dir := SessionDir(org, repo, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "comments.yaml")
	return WithLock(path, func() error {
		return writeYAML(path, comments)
	})
}

func writeYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// KeyForSession returns the on-disk key used to store the given session.
// Local sessions key on branch+base; PR sessions key on PR number.
func KeyForSession(s *Session) SessionKey {
	if s == nil {
		return ""
	}
	if s.Kind == SessionKindLocal {
		return LocalSessionKey(s.Branch, s.BaseBranch)
	}
	return PRSessionKey(s.PRNumber)
}

