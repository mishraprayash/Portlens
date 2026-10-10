package cmd

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/exitcode"
)

type mockSubcommand struct {
	name    string
	aliases []string
	ranWith []string
}

func (m *mockSubcommand) Name() string        { return m.name }
func (m *mockSubcommand) Aliases() []string   { return m.aliases }
func (m *mockSubcommand) Description() string { return "mock command" }
func (m *mockSubcommand) Run(_ context.Context, args []string, _ []string, _, _ io.Writer, _ io.Reader) int {
	m.ranWith = args
	return 42
}

func TestSubcommandRegistry(t *testing.T) {
	r := &SubcommandRegistry{commands: make(map[string]Subcommand)}
	cmd := &mockSubcommand{name: "test", aliases: []string{"t", "tst"}}
	r.Register(cmd)

	if got := r.Lookup("test"); got != cmd {
		t.Errorf("Lookup(test) = %v, want %v", got, cmd)
	}
	if got := r.Lookup("t"); got != cmd {
		t.Errorf("Lookup(t) = %v, want %v", got, cmd)
	}
	if got := r.Lookup("unknown"); got != nil {
		t.Errorf("Lookup(unknown) = %v, want nil", got)
	}
}

func TestExtractSubcommand(t *testing.T) {
	r := defaultSubcommandRegistry()

	tests := []struct {
		name       string
		args       []string
		wantCmd    string
		wantArgs   []string
		wantPreFlg []string
	}{
		{
			name:       "direct config command",
			args:       []string{"config", "list"},
			wantCmd:    "config",
			wantArgs:   []string{"list"},
			wantPreFlg: nil,
		},
		{
			name:       "config with leading flags",
			args:       []string{"--no-color", "--debug", "config", "path"},
			wantCmd:    "config",
			wantArgs:   []string{"path"},
			wantPreFlg: []string{"--no-color", "--debug"},
		},
		{
			name:       "kill subcommand",
			args:       []string{"kill", "3000", "--force"},
			wantCmd:    "kill",
			wantArgs:   []string{"3000", "--force"},
			wantPreFlg: nil,
		},
		{
			name:       "list alias ls",
			args:       []string{"ls", "--tcp"},
			wantCmd:    "list",
			wantArgs:   []string{"--tcp"},
			wantPreFlg: nil,
		},
		{
			name:       "no subcommand",
			args:       []string{"3000", "--json"},
			wantCmd:    "",
			wantArgs:   nil,
			wantPreFlg: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, subArgs, preFlags := extractSubcommand(tt.args, r)
			if tt.wantCmd == "" {
				if cmd != nil {
					t.Fatalf("expected nil cmd, got %v", cmd.Name())
				}
				return
			}
			if cmd == nil || cmd.Name() != tt.wantCmd {
				t.Fatalf("got cmd %v, want %s", cmd, tt.wantCmd)
			}
			if !reflect.DeepEqual(subArgs, tt.wantArgs) {
				t.Errorf("got subArgs %v, want %v", subArgs, tt.wantArgs)
			}
			if !reflect.DeepEqual(preFlags, tt.wantPreFlg) {
				t.Errorf("got preFlags %v, want %v", preFlags, tt.wantPreFlg)
			}
		})
	}
}

func TestExecuteSubcommandExecution(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"--no-color", "config", "path"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("Execute returned %d, want 0 (stderr: %s)", code, stderr.String())
	}
}

func TestSubcommandHelp(t *testing.T) {
	subcmds := []string{"list", "inspect", "kill", "restart", "open", "tree", "conn", "watch", "find", "next"}
	for _, sub := range subcmds {
		t.Run(sub+"_help", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Execute([]string{sub, "--help"}, &stdout, &stderr, nil)
			if code != 0 {
				t.Errorf("%s --help exited with %d", sub, code)
			}
			if !strings.Contains(stdout.String(), "portlens "+sub) && !strings.Contains(stdout.String(), "USAGE") {
				t.Errorf("%s --help missing usage:\n%s", sub, stdout.String())
			}
		})
	}
}

func TestSubcommandValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"kill without args", []string{"kill"}, 2},
		{"inspect without args", []string{"inspect"}, 2},
		{"restart without args", []string{"restart"}, 2},
		{"open without args", []string{"open"}, 2},
		{"tree without args", []string{"tree"}, 2},
		{"conn without args", []string{"conn"}, 2},
		{"find without args", []string{"find"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Execute(c.args, &stdout, &stderr, nil)
			if code != c.want {
				t.Errorf("Execute(%v) = %d, want %d (stderr: %s)", c.args, code, c.want, stderr.String())
			}
		})
	}
}

// Regression tests: flag values used to be mistaken for port targets, so
// invocations like `kill --filter node` printed the default listing and
// exited 0 without doing anything.
func TestSubcommandRejectsFlagValuesAsTargets(t *testing.T) {
	cases := [][]string{
		{"kill", "--filter", "node"},
		{"restart", "--filter", "node"},
		{"open", "--filter", "node"},
		{"tree", "--sort", "process"},
		{"conn", "--sort", "port"},
		{"inspect", "--protocol", "udp"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := Execute(args, &stdout, &stderr, nil)
		if code != exitcode.InvalidArguments {
			t.Errorf("Execute(%v) = %d, want %d (stderr: %s)", args, code, exitcode.InvalidArguments, stderr.String())
		}
	}
}

// Action flags without a port target must fail instead of silently degrading
// to the default listing.
func TestActionFlagsRequirePorts(t *testing.T) {
	cases := [][]string{
		{"--kill"},
		{"--restart"},
		{"--open"},
		{"--tree"},
		{"--connections"},
		{"--filter", "node", "--kill"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := Execute(args, &stdout, &stderr, nil)
		if code != exitcode.InvalidArguments {
			t.Errorf("Execute(%v) = %d, want %d (stderr: %s)", args, code, exitcode.InvalidArguments, stderr.String())
		}
	}
}

func TestSubcommandListExecution(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"list", "--no-color", "--json"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("Execute(list) = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "[") {
		t.Errorf("expected json array from list, got: %s", stdout.String())
	}
}

func TestFindSubcommandFlags(t *testing.T) {
	// Should not fail validation with exit code 2 (InvalidArguments)
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"find", "--name=nonexistent_dummy_process_12345"}, &stdout, &stderr, nil)
	// It should reach search execution and return PortNotFound (3) rather than InvalidArguments (2)
	if code != exitcode.PortNotFound {
		t.Errorf("Execute(find --name=...) = %d, want %d (stderr: %s)", code, exitcode.PortNotFound, stderr.String())
	}
}

// Watch mode only monitors: combining it with an action flag must fail with
// exit 2 instead of silently ignoring the action. The timeout guarantees the
// test fails rather than hangs if validation ever regresses.
func TestWatchRejectsActionFlags(t *testing.T) {
	cases := [][]string{
		{"--watch", "--kill", "3000"},
		{"-w", "-k", "3000"},
		{"watch", "--kill", "3000"},
		{"watch", "--restart", "3000"},
		{"--watch", "--tree", "3000"},
	}
	for _, args := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var stdout, stderr bytes.Buffer
		code := ExecuteContext(ctx, args, &stdout, &stderr, nil)
		cancel()
		if code != exitcode.InvalidArguments {
			t.Errorf("Execute(%v) = %d, want %d (stderr: %s)", args, code, exitcode.InvalidArguments, stderr.String())
		}
		if !strings.Contains(stderr.String(), "--watch cannot be combined") {
			t.Errorf("Execute(%v) stderr = %q, want --watch cannot be combined message", args, stderr.String())
		}
	}
}
