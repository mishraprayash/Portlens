package cmd

import (
	"context"
	"io"
	"strings"
)

// Subcommand defines the interface for modular top-level CLI subcommands.
type Subcommand interface {
	Name() string
	Aliases() []string
	Description() string
	Run(ctx context.Context, args []string, stdout, stderr io.Writer, stdin io.Reader) int
}

// SubcommandRegistry manages available subcommands and their aliases.
type SubcommandRegistry struct {
	commands map[string]Subcommand
}

func defaultSubcommandRegistry() *SubcommandRegistry {
	r := &SubcommandRegistry{commands: make(map[string]Subcommand)}
	r.Register(&configSubcommand{})
	r.Register(&freeSubcommand{})
	r.Register(&nextSubcommand{})
	r.Register(&completionSubcommand{})
	return r
}

// Register adds a subcommand and any aliases to the registry.
func (r *SubcommandRegistry) Register(cmd Subcommand) {
	r.commands[cmd.Name()] = cmd
	for _, alias := range cmd.Aliases() {
		r.commands[alias] = cmd
	}
}

// Lookup finds a subcommand by primary name or alias.
func (r *SubcommandRegistry) Lookup(name string) Subcommand {
	return r.commands[name]
}

// configSubcommand handles `portlens config ...`.
type configSubcommand struct{}

func (c *configSubcommand) Name() string        { return "config" }
func (c *configSubcommand) Aliases() []string   { return nil }
func (c *configSubcommand) Description() string { return "Manage named port groups (@name)" }
func (c *configSubcommand) Run(_ context.Context, args []string, stdout, stderr io.Writer, _ io.Reader) int {
	return runConfig(args, stdout, stderr)
}

// extractSubcommand scans args to see if a registered subcommand is the first
// positional command, returning the matched subcommand, remaining arguments,
// and any preceding global flags.
func extractSubcommand(args []string, registry *SubcommandRegistry) (Subcommand, []string, []string) {
	var preFlags []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			preFlags = append(preFlags, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			}
			if valueFlags[name] && !strings.Contains(a, "=") && i+1 < len(args) {
				preFlags = append(preFlags, args[i+1])
				i++
			}
			continue
		}
		if cmd := registry.Lookup(a); cmd != nil {
			return cmd, args[i+1:], preFlags
		}
		break
	}
	return nil, nil, nil
}
