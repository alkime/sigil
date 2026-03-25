package cli_test

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/alkime/sigil/cli"
)

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", "0001"},
		{"0001", "0001"},
		{"42", "0042"},
		{"abc", "0abc"},
		{"0", "0000"},
		{"999", "0999"},
		{"  1  ", "0001"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cli.NormalizeID(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestKongParsing(t *testing.T) {
	t.Run("empty args routes to TUI", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c,
			kong.Name("sigil"),
			kong.Exit(func(int) {}),
		)
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		// No args at all — should fail because TUI requires a file arg
		// but the default command should be selected.
		ctx, err := p.Parse([]string{"somefile.md"})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		cmd := ctx.Command()
		if cmd != "tui <file>" {
			t.Errorf("expected default TUI command, got %q", cmd)
		}
	})

	t.Run("get-comments routes to GetComments", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c, kong.Name("sigil"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		ctx, err := p.Parse([]string{"get-comments", "file.md"})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		cmd := ctx.Command()
		if cmd != "get-comments <file>" {
			t.Errorf("expected 'get-comments <file>', got %q", cmd)
		}
	})

	t.Run("subcommand missing required arg errors", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c, kong.Name("sigil"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		// get-comments requires a file argument; omitting it should error.
		_, err = p.Parse([]string{"get-comments"})
		if err == nil {
			t.Error("expected error for missing required arg, got nil")
		}
	})

	t.Run("no args errors", func(t *testing.T) {
		var c cli.CLI
		p, err := kong.New(&c, kong.Name("sigil"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		// No args at all — TUI default requires a file arg.
		_, err = p.Parse([]string{})
		if err == nil {
			t.Error("expected error for no args, got nil")
		}
	})
}
