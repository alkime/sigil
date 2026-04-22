package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/alkime/sigil/diff"
)

// DiffListSessionsCmd prints locally-persisted sigil diff sessions so agents
// can pick one for the --session flag on other diff subcommands.
type DiffListSessionsCmd struct {
	All bool `help:"List sessions across all repos, not just the current one."`
}

// sessionRow holds the fields we print per session — assembled from the
// sessions index plus the session YAML itself.
type sessionRow struct {
	Repo      string
	PRNumber  int
	Branch    string
	Title     string
	HeadSHA   string
	SessionID string
	UpdatedAt time.Time
}

func (c *DiffListSessionsCmd) Run(ctx *CLIContext) error {
	var repoFilter string
	if !c.All {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		_, owner, repo, err := diff.OriginRemote(context.Background(), cwd)
		if err != nil {
			return fmt.Errorf("auto-detect repo (use --all to list globally): %w", err)
		}
		repoFilter = owner + "/" + repo
	}

	rows, err := collectSessionRows(repoFilter)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		if repoFilter != "" {
			fmt.Fprintf(ctx.Out, "no sessions for %s (try --all)\n", repoFilter)
		} else {
			fmt.Fprintln(ctx.Out, "no sessions found")
		}
		return nil
	}

	// Most recent first — agents typically want the live PR at the top.
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
	})

	tw := tabwriter.NewWriter(ctx.Out, 0, 0, 2, ' ', 0)
	if c.All {
		fmt.Fprintln(tw, "REPO\tPR\tBRANCH\tTITLE\tUPDATED\tSESSION")
	} else {
		fmt.Fprintln(tw, "PR\tBRANCH\tTITLE\tUPDATED\tSESSION")
	}
	for _, r := range rows {
		updated := r.UpdatedAt.UTC().Format(time.RFC3339)
		title := truncate(r.Title, 60)
		if c.All {
			fmt.Fprintf(tw, "%s\t#%d\t%s\t%s\t%s\t%s\n",
				r.Repo, r.PRNumber, r.Branch, title, updated, r.SessionID)
		} else {
			fmt.Fprintf(tw, "#%d\t%s\t%s\t%s\t%s\n",
				r.PRNumber, r.Branch, title, updated, r.SessionID)
		}
	}
	return tw.Flush()
}

// collectSessionRows walks the sigil diff base path and loads every session
// referenced in each repo's sessions.yaml index. When repoFilter is non-empty
// (format "owner/name") only that repo's sessions are returned. Sessions
// whose YAML is missing or malformed are silently skipped.
func collectSessionRows(repoFilter string) ([]sessionRow, error) {
	base := diff.BasePath()
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil
	}

	pattern := filepath.Join(base, "*", "*", "sessions.yaml")
	indexFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob sessions: %w", err)
	}

	var rows []sessionRow
	for _, idxPath := range indexFiles {
		// {base}/{org}/{repo}/sessions.yaml
		repoDir := filepath.Dir(idxPath)
		repoName := filepath.Base(repoDir)
		org := filepath.Base(filepath.Dir(repoDir))
		repoFull := org + "/" + repoName

		if repoFilter != "" && repoFull != repoFilter {
			continue
		}

		entries, err := diff.LoadIndex(org, repoName)
		if err != nil {
			continue
		}
		for _, e := range entries {
			session, err := diff.LoadSession(org, repoName, e.PRNumber)
			if err != nil || session == nil {
				continue
			}
			rows = append(rows, sessionRow{
				Repo:      repoFull,
				PRNumber:  session.PRNumber,
				Branch:    session.Branch,
				Title:     session.PRTitle,
				HeadSHA:   session.HeadSHA,
				SessionID: session.ID,
				UpdatedAt: session.UpdatedAt,
			})
		}
	}
	return rows, nil
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
