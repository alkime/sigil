package cli

import (
	"context"
	"os"

	"github.com/alkime/sigil/diff"
	dtui "github.com/alkime/sigil/diff/tui"
)

// DiffCmd is the parent command for all sigil diff sub-subcommands.
type DiffCmd struct {
	Session      string               `short:"s" help:"Use specific session ID (skip auto-detect)."`
	Draft        bool                 `help:"Include draft PRs in auto-detect."`
	GetComments  DiffGetCommentsCmd   `cmd:"" name:"get-comments" help:"Print comments on the current PR review session."`
	ListSessions DiffListSessionsCmd  `cmd:"" name:"list-sessions" help:"List locally-persisted review sessions (current repo by default)."`
	ReplyComment DiffReplyCommentCmd  `cmd:"" name:"reply-comment" help:"Append a reply to a comment."`
	Resolve      DiffResolveCmd       `cmd:"" name:"resolve-comments" aliases:"resolve-comment" help:"Mark comments as resolved."`
	Unresolve    DiffUnresolveCmd     `cmd:"" name:"unresolve-comments" aliases:"unresolve-comment" help:"Mark comments as unresolved."`
	TUI          DiffTUICmd           `cmd:"" default:"withargs" hidden:""`
}

// AfterApply propagates DiffCmd-level flags to CLIContext for sub-subcommands.
func (c *DiffCmd) AfterApply(ctx *CLIContext) error {
	ctx.DiffSession = c.Session
	ctx.DiffDraft = c.Draft
	return nil
}

// DiffTUICmd is the default subcommand: launches the diff TUI.
type DiffTUICmd struct{}

func (c *DiffTUICmd) Run(ctx *CLIContext) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	opts := diff.ResolveOpts{
		SessionID:    ctx.DiffSession,
		IncludeDraft: ctx.DiffDraft,
		CWD:          cwd,
	}
	return dtui.RunWithResolve(context.Background(), opts)
}
