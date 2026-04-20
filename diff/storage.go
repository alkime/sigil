package diff

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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

func SessionDir(org, repo string, prNumber int) string {
	return filepath.Join(BasePath(), org, repo, fmt.Sprintf("%d", prNumber))
}

func SnapshotDir(org, repo string, prNumber int, baseSHA, headSHA string) string {
	return filepath.Join(SessionDir(org, repo, prNumber), "snapshots", baseSHA+"_"+headSHA)
}

func LoadSession(org, repo string, prNumber int) (*Session, error) {
	path := filepath.Join(SessionDir(org, repo, prNumber), "session.yaml")
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

func SaveSession(org, repo string, prNumber int, s *Session) error {
	dir := SessionDir(org, repo, prNumber)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeYAML(filepath.Join(dir, "session.yaml"), s)
}

func LoadComments(org, repo string, prNumber int) ([]Comment, error) {
	path := filepath.Join(SessionDir(org, repo, prNumber), "comments.yaml")
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

func SaveComments(org, repo string, prNumber int, comments []Comment) error {
	dir := SessionDir(org, repo, prNumber)
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
