package cmd

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/mishraprayash/Portlens/internal/exitcode"
)

// Subcommand defines the interface for modular top-level CLI subcommands.
type Subcommand interface {
	Name() string
	Aliases() []string
	Description() string
	Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int
}

// SubcommandRegistry manages available subcommands and their aliases.
type SubcommandRegistry struct {
	commands map[string]Subcommand
	ordered  []Subcommand // canonical commands in registration order
}

func defaultSubcommandRegistry() *SubcommandRegistry {
	r := &SubcommandRegistry{commands: make(map[string]Subcommand)}
	r.Register(&listSubcommand{})
	r.Register(&inspectSubcommand{})
	r.Register(&killSubcommand{})
	r.Register(&restartSubcommand{})
	r.Register(&openSubcommand{})
	r.Register(&treeSubcommand{})
	r.Register(&connSubcommand{})
	r.Register(&watchSubcommand{})
	r.Register(&findSubcommand{})
	r.Register(&nextSubcommand{})
	r.Register(&topSubcommand{})
	r.Register(&configSubcommand{})
	r.Register(&completionSubcommand{})
	return r
}

// Register adds a subcommand and any aliases to the registry.
func (r *SubcommandRegistry) Register(cmd Subcommand) {
	if _, seen := r.commands[cmd.Name()]; !seen {
		r.ordered = append(r.ordered, cmd)
	}
	r.commands[cmd.Name()] = cmd
	for _, alias := range cmd.Aliases() {
		r.commands[alias] = cmd
	}
}

// Lookup finds a subcommand by primary name or alias.
func (r *SubcommandRegistry) Lookup(name string) Subcommand {
	return r.commands[name]
}

// wantsHelp reports whether the arguments ask for the command's help
// (subcommand form: `portlens <cmd> help` as well as --help/-h).
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			return true
		}
	}
	return false
}

// simpleOpts declares what a straightforward subcommand needs to run: its
// help text, the action flag executeCore should act on (if any), and an
// optional port-target requirement.
type simpleOpts struct {
	usage         func(io.Writer)
	actionFlag    string // prepended before args, e.g. "--kill"; "" = plain listing
	portTargetMsg string // fail message when no port target is present; "" = target not required
}

// runSimple is the shared body of the subcommands that only check for help,
// optionally require a port target, and forward to executeCore.
func runSimple(ctx context.Context, args, preFlags []string, stdout, stderr io.Writer, stdin io.Reader, o simpleOpts) int {
	if wantsHelp(args) {
		o.usage(stdout)
		return exitcode.Success
	}
	targets := append(append([]string(nil), preFlags...), args...)
	if o.portTargetMsg != "" && !hasPortTarget(targets) {
		return fail(stderr, exitcode.InvalidArguments, "%s", o.portTargetMsg)
	}
	if o.actionFlag != "" {
		targets = append(append([]string(nil), preFlags...), append([]string{o.actionFlag}, args...)...)
	}
	return executeCore(ctx, targets, stdout, stderr, stdin)
}

// configSubcommand handles `portlens config ...`.
type configSubcommand struct{}

func (c *configSubcommand) Name() string        { return "config" }
func (c *configSubcommand) Aliases() []string   { return nil }
func (c *configSubcommand) Description() string { return "Manage named port groups (@name)" }
func (c *configSubcommand) Run(_ context.Context, args []string, _ []string, stdout, stderr io.Writer, _ io.Reader) int {
	return runConfig(args, stdout, stderr)
}

// listSubcommand handles `portlens list [flags]` and `portlens ls`.
type listSubcommand struct{}

func (c *listSubcommand) Name() string        { return "list" }
func (c *listSubcommand) Aliases() []string   { return []string{"ls"} }
func (c *listSubcommand) Description() string { return "List active listening ports" }
func (c *listSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{usage: printListUsage})
}

// inspectSubcommand handles `portlens inspect <port...> [flags]`.
type inspectSubcommand struct{}

func (c *inspectSubcommand) Name() string      { return "inspect" }
func (c *inspectSubcommand) Aliases() []string { return nil }
func (c *inspectSubcommand) Description() string {
	return "Inspect port(s) with process details and exposure"
}
func (c *inspectSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{
		usage:         printInspectUsage,
		portTargetMsg: "portlens inspect: specify one or more ports to inspect\nRun 'portlens inspect --help' for usage.\n",
	})
}

// killSubcommand handles `portlens kill <port...> [flags]`.
type killSubcommand struct{}

func (c *killSubcommand) Name() string      { return "kill" }
func (c *killSubcommand) Aliases() []string { return nil }
func (c *killSubcommand) Description() string {
	return "Gracefully terminate process on port(s) (SIGTERM)"
}
func (c *killSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{
		usage:         printKillUsage,
		actionFlag:    "--kill",
		portTargetMsg: "portlens kill: specify port(s) to terminate or use --all\nRun 'portlens kill --help' for usage.\n",
	})
}

// restartSubcommand handles `portlens restart <port> [flags]`.
type restartSubcommand struct{}

func (c *restartSubcommand) Name() string        { return "restart" }
func (c *restartSubcommand) Aliases() []string   { return nil }
func (c *restartSubcommand) Description() string { return "Restart process if launch command is known" }
func (c *restartSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{
		usage:         printRestartUsage,
		actionFlag:    "--restart",
		portTargetMsg: "portlens restart: specify a port to restart\nRun 'portlens restart --help' for usage.\n",
	})
}

// openSubcommand handles `portlens open <port> [flags]`.
type openSubcommand struct{}

func (c *openSubcommand) Name() string        { return "open" }
func (c *openSubcommand) Aliases() []string   { return nil }
func (c *openSubcommand) Description() string { return "Open service in your default web browser" }
func (c *openSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{
		usage:         printOpenUsage,
		actionFlag:    "--open",
		portTargetMsg: "portlens open: specify a port to open\nRun 'portlens open --help' for usage.\n",
	})
}

// treeSubcommand handles `portlens tree <port> [flags]`.
type treeSubcommand struct{}

func (c *treeSubcommand) Name() string      { return "tree" }
func (c *treeSubcommand) Aliases() []string { return nil }
func (c *treeSubcommand) Description() string {
	return "Display process ancestor and descendant hierarchy"
}
func (c *treeSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{
		usage:         printTreeUsage,
		actionFlag:    "--tree",
		portTargetMsg: "portlens tree: specify a port\nRun 'portlens tree --help' for usage.\n",
	})
}

// connSubcommand handles `portlens conn <port> [flags]`.
type connSubcommand struct{}

func (c *connSubcommand) Name() string      { return "conn" }
func (c *connSubcommand) Aliases() []string { return []string{"connections"} }
func (c *connSubcommand) Description() string {
	return "Show active network connections for the process"
}
func (c *connSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{
		usage:         printConnUsage,
		actionFlag:    "--connections",
		portTargetMsg: "portlens conn: specify a port\nRun 'portlens conn --help' for usage.\n",
	})
}

// watchSubcommand handles `portlens watch [port...] [flags]`.
type watchSubcommand struct{}

func (c *watchSubcommand) Name() string      { return "watch" }
func (c *watchSubcommand) Aliases() []string { return nil }
func (c *watchSubcommand) Description() string {
	return "Live-monitor port states with desktop notifications"
}
func (c *watchSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runSimple(ctx, args, preFlags, stdout, stderr, stdin, simpleOpts{
		usage:      printWatchUsage,
		actionFlag: "--watch",
	})
}

// findSubcommand handles `portlens find <query> [flags]`.
type findSubcommand struct{}

func (c *findSubcommand) Name() string        { return "find" }
func (c *findSubcommand) Aliases() []string   { return nil }
func (c *findSubcommand) Description() string { return "Find ports by process name/command or PID" }
func (c *findSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	if wantsHelp(args) {
		printFindUsage(stdout)
		return exitcode.Success
	}
	var extraFlags []string
	var query string
	hasTarget := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--pid" && i+1 < len(args) {
			hasTarget = true
			extraFlags = append(extraFlags, a, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(a, "--pid=") {
			hasTarget = true
			extraFlags = append(extraFlags, a)
			continue
		}
		if a == "--name" && i+1 < len(args) {
			hasTarget = true
			extraFlags = append(extraFlags, a, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(a, "--name=") {
			hasTarget = true
			extraFlags = append(extraFlags, a)
			continue
		}
		if strings.HasPrefix(a, "-") {
			extraFlags = append(extraFlags, a)
			continue
		}
		if query == "" {
			query = a
		} else {
			extraFlags = append(extraFlags, a)
		}
	}
	if !hasTarget && query == "" {
		return fail(stderr, exitcode.InvalidArguments, "portlens find: specify process name/query or --pid <pid>\nRun 'portlens find --help' for usage.\n")
	}
	if query != "" && !hasTarget {
		if p, err := strconv.Atoi(query); err == nil && p > 0 {
			extraFlags = append([]string{"--pid", query}, extraFlags...)
		} else {
			extraFlags = append([]string{"--name", query}, extraFlags...)
		}
	}
	return executeCore(ctx, append(preFlags, extraFlags...), stdout, stderr, stdin)
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
