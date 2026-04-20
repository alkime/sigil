package diff

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func indexPath(org, repo string) string {
	return filepath.Join(BasePath(), org, repo, "sessions.yaml")
}

func LoadIndex(org, repo string) ([]SessionsIndexEntry, error) {
	data, err := os.ReadFile(indexPath(org, repo))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []SessionsIndexEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func SaveIndex(org, repo string, entries []SessionsIndexEntry) error {
	dir := filepath.Join(BasePath(), org, repo)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeYAML(indexPath(org, repo), entries)
}
