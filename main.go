package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/alkime/sigil/cli"
)

// Populated at release time by GoReleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var c cli.CLI
	cliCtx := &cli.CLIContext{Out: os.Stdout}
	ctx := kong.Parse(&c,
		kong.Name("sigil"),
		kong.Description("Terminal Markdown viewer with inline review commenting."),
		kong.Vars{"version": fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)},
		kong.Bind(cliCtx),
	)
	err := ctx.Run(cliCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
