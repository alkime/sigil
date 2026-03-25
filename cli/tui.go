package cli

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/alkime/sigil/model"
	"github.com/alkime/sigil/parser"
	"github.com/alkime/sigil/writer"
)

// TUICmd is the default command that launches the interactive TUI.
type TUICmd struct {
	File string `arg:"" help:"Markdown file to view."`
}

// Run launches the Bubbletea TUI for the given Markdown file.
func (c *TUICmd) Run(_ *CLIContext) error {
	doc, err := parser.Parse(c.File)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	app := model.NewApp(doc, writer.WriteComment, writer.UpdateComment, writer.DeleteComment)
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
