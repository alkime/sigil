package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alkime/sigil/diff"
)

// DiffReplyCommentCmd appends a reply to an existing diff comment.
type DiffReplyCommentCmd struct {
	ID        string `arg:"" help:"Comment ID to reply to."`
	ReplyText string `arg:"" name:"reply" help:"Reply text."`
	Author    string `help:"Override author (default: git config user.name or $USER)."`
}

// Run appends "\n\nREPLY: <text>" to the comment body and saves under flock.
func (c *DiffReplyCommentCmd) Run(ctx *CLIContext) error {
	session, _, err := diff.Resolve(context.Background(), diff.ResolveOpts{
		SessionID:    ctx.DiffSession,
		IncludeDraft: ctx.DiffDraft,
	})
	if err != nil {
		return fmt.Errorf("resolve session: %w", err)
	}

	org, repo := splitRepo(session.Repo)
	comments, err := diff.LoadComments(org, repo, session.PRNumber)
	if err != nil {
		return fmt.Errorf("load comments: %w", err)
	}

	found := false
	for i := range comments {
		if comments[i].ID == c.ID {
			comments[i].Body = comments[i].Body + "\n\nREPLY: " + c.ReplyText
			comments[i].UpdatedAt = time.Now().UTC()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("comment %s not found", c.ID)
	}

	if err := diff.SaveComments(org, repo, session.PRNumber, comments); err != nil {
		return fmt.Errorf("save comments: %w", err)
	}

	fmt.Fprintf(ctx.Out, "Replied to %s\n", c.ID)
	return nil
}

func defaultAuthor() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "unknown"
}
