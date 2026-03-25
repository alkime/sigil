package cli

import (
	"fmt"
	"io"
	"strings"
)

// CLI is the top-level Kong command structure for Sigil.
type CLI struct {
	GetComments       GetCommentsCmd       `cmd:"" name:"get-comments" help:"Print comments from a Markdown file as JSON."`
	ResolveComments   ResolveCommentsCmd   `cmd:"" name:"resolve-comments" aliases:"resolve-comment" help:"Mark comments as resolved."`
	UnresolveComments UnresolveCommentsCmd `cmd:"" name:"unresolve-comments" aliases:"unresolve-comment" help:"Mark comments as unresolved."`
	ReplyComment      ReplyCommentCmd      `cmd:"" name:"reply-comment" help:"Add a reply to a comment."`
	GenerateSkill     GenerateSkillCmd     `cmd:"" name:"generate-skill" help:"Print LLM skill doc to stdout (e.g. sigil generate-skill > SKILL.md)."`
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
	File       string `arg:"" help:"Markdown file to read."`
	Open       bool   `help:"Show only open comments." xor:"status"`
	Resolved   bool   `help:"Show only resolved comments." xor:"status"`
}

// ResolveCommentsCmd marks comments as resolved.
type ResolveCommentsCmd struct {
	File string   `arg:"" help:"Markdown file to update."`
	IDs  []string `arg:"" help:"Comment IDs to resolve."`
}

// UnresolveCommentsCmd marks comments as unresolved.
type UnresolveCommentsCmd struct {
	File string   `arg:"" help:"Markdown file to update."`
	IDs  []string `arg:"" help:"Comment IDs to unresolve."`
}

// ReplyCommentCmd adds a reply to a comment.
type ReplyCommentCmd struct {
	File      string `arg:"" help:"Markdown file to update."`
	ID        string `arg:"" help:"Comment ID to reply to."`
	ReplyText string `arg:"" name:"reply" help:"Reply text."`
}

// GenerateSkillCmd prints a Sigil skill document to stdout.
type GenerateSkillCmd struct{}
