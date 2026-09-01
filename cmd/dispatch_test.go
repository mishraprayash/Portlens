package cmd

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
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
