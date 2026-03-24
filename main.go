package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/alkime/sigil/model"
	"github.com/alkime/sigil/parser"
	"github.com/alkime/sigil/writer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sigil <file.md>")
		os.Exit(1)
	}

	doc, err := parser.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	app := model.NewApp(doc, writer.WriteComment)
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
