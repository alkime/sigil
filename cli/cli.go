package cli

import (
	"fmt"
	"io"
	"strings"
)

// CLI is the top-level Kong command structure for Sigil.
type CLI struct {
	GetComments       GetCommentsCmd       `cmd:"" name:"get-comments" help:"Print comments from a Markdown file as JSON."`
	ResolveComments   ResolveCommentsCmd   `cmd:"" name:"resolve-comments" help:"Mark comments as resolved."`
	UnresolveComments UnresolveCommentsCmd `cmd:"" name:"unresolve-comments" help:"Mark comments as unresolved."`
	ReplyComment      ReplyCommentCmd      `cmd:"" name:"reply-comment" help:"Add a reply to a comment."`
	InstallSkill      InstallSkillCmd      `cmd:"" name:"install-skill" help:"Install a Sigil skill file."`
	TUI               TUICmd               `cmd:"" default:"withargs" hidden:""`
}

// CLIContext carries shared dependencies for all subcommands.
type CLIContext struct {
	Out io.Writer
}

// NormalizeID converts a bare integer or short string into a zero-padded
// 4-digit review-comment ID. For example "1" -> "0001", "42" -> "0042".
// An already-padded ID like "0001" passes through unchanged.
func NormalizeID(raw string) string {
	raw = strings.TrimSpace(raw)
	// Strip leading zeros so we can re-pad uniformly.
	stripped := strings.TrimLeft(raw, "0")
	if stripped == "" {
		stripped = "0"
	}
	return fmt.Sprintf("%04s", stripped)
}

// --- Stub subcommand structs ---

// GetCommentsCmd prints review comments from a file.
type GetCommentsCmd struct {
	File string `arg:"" help:"Markdown file to read."`
}

func (c *GetCommentsCmd) Run(_ *CLIContext) error { return nil }

// ResolveCommentsCmd marks comments as resolved.
type ResolveCommentsCmd struct {
	File string   `arg:"" help:"Markdown file to update."`
	IDs  []string `arg:"" help:"Comment IDs to resolve."`
}

func (c *ResolveCommentsCmd) Run(_ *CLIContext) error { return nil }

// UnresolveCommentsCmd marks comments as unresolved.
type UnresolveCommentsCmd struct {
	File string   `arg:"" help:"Markdown file to update."`
	IDs  []string `arg:"" help:"Comment IDs to unresolve."`
}

func (c *UnresolveCommentsCmd) Run(_ *CLIContext) error { return nil }

// ReplyCommentCmd adds a reply to a comment.
type ReplyCommentCmd struct {
	File  string `arg:"" help:"Markdown file to update."`
	ID    string `arg:"" help:"Comment ID to reply to."`
	Reply string `arg:"" help:"Reply text."`
}

func (c *ReplyCommentCmd) Run(_ *CLIContext) error { return nil }

// InstallSkillCmd installs a Sigil skill file.
type InstallSkillCmd struct {
	Path string `arg:"" help:"Path to skill file."`
}

func (c *InstallSkillCmd) Run(_ *CLIContext) error { return nil }
