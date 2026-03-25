package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/alkime/sigil/cli"
)

func main() {
	var c cli.CLI
	ctx := kong.Parse(&c, kong.Name("sigil"), kong.Description("Terminal Markdown viewer with inline review commenting."))
	err := ctx.Run(&cli.CLIContext{Out: os.Stdout})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
