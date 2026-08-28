package cmd

import (
	"bytes"
	"context"
	"io"
	"reflect"
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
func (m *mockSubcommand) Run(_ context.Context, args []string, _, _ io.Writer, _ io.Reader) int {
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
			args:       []string{"--no-color", "--log", "out.log", "config", "path"},
			wantCmd:    "config",
			wantArgs:   []string{"path"},
			wantPreFlg: []string{"--no-color", "--log", "out.log"},
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
