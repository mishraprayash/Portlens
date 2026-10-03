package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mishraprayash/Portlens/internal/exitcode"
)

func TestTopSubcommandHelp(t *testing.T) {
	cmd := &topSubcommand{}
	if cmd.Name() != "top" {
		t.Errorf("expected name 'top', got %q", cmd.Name())
	}
	if len(cmd.Aliases()) == 0 || cmd.Aliases()[0] != "tui" {
		t.Errorf("expected alias 'tui', got %v", cmd.Aliases())
	}

	var stdout, stderr bytes.Buffer
	code := cmd.Run(context.Background(), []string{"--help"}, nil, &stdout, &stderr, strings.NewReader(""))
	if code != exitcode.Success {
		t.Errorf("expected success exit code for --help, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "portlens top") || !strings.Contains(out, "KEYBINDINGS") {
		t.Errorf("expected usage help in stdout: %s", out)
	}
}

func TestTopSubcommandNonTerminal(t *testing.T) {
	cmd := &topSubcommand{}
	var stdout, stderr bytes.Buffer
	// bytes.Buffer is not a terminal *os.File, so it should fail gracefully
	code := cmd.Run(context.Background(), []string{"--interval=1"}, nil, &stdout, &stderr, strings.NewReader(""))
	if code == exitcode.Success {
		t.Errorf("expected failure when running top in non-interactive environment")
	}
}

// Unknown flags and stray arguments used to be silently dropped, so typos
// like `portlens top --interval3` quietly ran with defaults.
func TestTopRejectsUnknownFlags(t *testing.T) {
	cmd := &topSubcommand{}
	var stdout, stderr bytes.Buffer
	code := cmd.Run(context.Background(), []string{"--filter", "node"}, nil, &stdout, &stderr, strings.NewReader(""))
	if code != exitcode.InvalidArguments {
		t.Errorf("expected %d for unknown flag, got %d", exitcode.InvalidArguments, code)
	}
	if !strings.Contains(stderr.String(), "--filter") {
		t.Errorf("expected error to name the offending flag, got: %s", stderr.String())
	}
}

func TestTopRejectsStrayArgument(t *testing.T) {
	cmd := &topSubcommand{}
	var stdout, stderr bytes.Buffer
	code := cmd.Run(context.Background(), []string{"bogus"}, nil, &stdout, &stderr, strings.NewReader(""))
	if code != exitcode.InvalidArguments {
		t.Errorf("expected %d for stray argument, got %d", exitcode.InvalidArguments, code)
	}
}

func TestDispatchTopRegistry(t *testing.T) {
	reg := defaultSubcommandRegistry()
	if cmd := reg.Lookup("top"); cmd == nil {
		t.Errorf("expected 'top' subcommand in registry")
	}
	if cmd := reg.Lookup("tui"); cmd == nil {
		t.Errorf("expected 'tui' alias in registry")
	}
	if cmd := reg.Lookup("dashboard"); cmd == nil {
		t.Errorf("expected 'dashboard' alias in registry")
	}
}
