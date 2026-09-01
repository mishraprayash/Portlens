package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/portlens/portlens/internal/exitcode"
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
	r.Register(&configSubcommand{})
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
func (c *configSubcommand) Run(_ context.Context, args []string, _ []string, stdout, stderr io.Writer, _ io.Reader) int {
	return runConfig(args, stdout, stderr)
}

// listSubcommand handles `portlens list [flags]` and `portlens ls`.
type listSubcommand struct{}

func (c *listSubcommand) Name() string        { return "list" }
func (c *listSubcommand) Aliases() []string   { return []string{"ls"} }
func (c *listSubcommand) Description() string { return "List active listening ports" }
func (c *listSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printListUsage(stdout)
			return exitcode.Success
		}
	}
	return executeCore(ctx, append(preFlags, args...), stdout, stderr, stdin)
}

// inspectSubcommand handles `portlens inspect <port...> [flags]`.
type inspectSubcommand struct{}

func (c *inspectSubcommand) Name() string      { return "inspect" }
func (c *inspectSubcommand) Aliases() []string { return []string{"info", "show"} }
func (c *inspectSubcommand) Description() string {
	return "Inspect port(s) with process details and exposure"
}
func (c *inspectSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printInspectUsage(stdout)
			return exitcode.Success
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "portlens inspect: specify one or more ports to inspect")
		fmt.Fprintln(stderr, "Run 'portlens inspect --help' for usage.")
		return exitcode.InvalidArguments
	}
	return executeCore(ctx, append(preFlags, args...), stdout, stderr, stdin)
}

// killSubcommand handles `portlens kill <port...> [flags]`.
type killSubcommand struct{}

func (c *killSubcommand) Name() string      { return "kill" }
func (c *killSubcommand) Aliases() []string { return []string{"stop", "term"} }
func (c *killSubcommand) Description() string {
	return "Gracefully terminate process on port(s) (SIGTERM)"
}
func (c *killSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printKillUsage(stdout)
			return exitcode.Success
		}
	}
	hasTarget := false
	for _, a := range append(preFlags, args...) {
		if a == "--all" || (!strings.HasPrefix(a, "-") && a != "") {
			hasTarget = true
			break
		}
	}
	if !hasTarget {
		fmt.Fprintln(stderr, "portlens kill: specify port(s) to terminate or use --all")
		fmt.Fprintln(stderr, "Run 'portlens kill --help' for usage.")
		return exitcode.InvalidArguments
	}
	return executeCore(ctx, append(preFlags, append([]string{"--kill"}, args...)...), stdout, stderr, stdin)
}

// restartSubcommand handles `portlens restart <port> [flags]`.
type restartSubcommand struct{}

func (c *restartSubcommand) Name() string        { return "restart" }
func (c *restartSubcommand) Aliases() []string   { return nil }
func (c *restartSubcommand) Description() string { return "Restart process if launch command is known" }
func (c *restartSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printRestartUsage(stdout)
			return exitcode.Success
		}
	}
	hasPort := false
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			hasPort = true
			break
		}
	}
	if !hasPort {
		fmt.Fprintln(stderr, "portlens restart: specify a port to restart")
		fmt.Fprintln(stderr, "Run 'portlens restart --help' for usage.")
		return exitcode.InvalidArguments
	}
	return executeCore(ctx, append(preFlags, append([]string{"--restart"}, args...)...), stdout, stderr, stdin)
}

// openSubcommand handles `portlens open <port> [flags]`.
type openSubcommand struct{}

func (c *openSubcommand) Name() string        { return "open" }
func (c *openSubcommand) Aliases() []string   { return nil }
func (c *openSubcommand) Description() string { return "Open service in your default web browser" }
func (c *openSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printOpenUsage(stdout)
			return exitcode.Success
		}
	}
	hasPort := false
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			hasPort = true
			break
		}
	}
	if !hasPort {
		fmt.Fprintln(stderr, "portlens open: specify a port to open")
		fmt.Fprintln(stderr, "Run 'portlens open --help' for usage.")
		return exitcode.InvalidArguments
	}
	return executeCore(ctx, append(preFlags, append([]string{"--open"}, args...)...), stdout, stderr, stdin)
}

// treeSubcommand handles `portlens tree <port> [flags]`.
type treeSubcommand struct{}

func (c *treeSubcommand) Name() string      { return "tree" }
func (c *treeSubcommand) Aliases() []string { return nil }
func (c *treeSubcommand) Description() string {
	return "Display process ancestor and descendant hierarchy"
}
func (c *treeSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printTreeUsage(stdout)
			return exitcode.Success
		}
	}
	hasPort := false
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			hasPort = true
			break
		}
	}
	if !hasPort {
		fmt.Fprintln(stderr, "portlens tree: specify a port")
		fmt.Fprintln(stderr, "Run 'portlens tree --help' for usage.")
		return exitcode.InvalidArguments
	}
	return executeCore(ctx, append(preFlags, append([]string{"--tree"}, args...)...), stdout, stderr, stdin)
}

// connSubcommand handles `portlens conn <port> [flags]`.
type connSubcommand struct{}

func (c *connSubcommand) Name() string      { return "conn" }
func (c *connSubcommand) Aliases() []string { return []string{"connections", "net"} }
func (c *connSubcommand) Description() string {
	return "Show active network connections for the process"
}
func (c *connSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printConnUsage(stdout)
			return exitcode.Success
		}
	}
	hasPort := false
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			hasPort = true
			break
		}
	}
	if !hasPort {
		fmt.Fprintln(stderr, "portlens conn: specify a port")
		fmt.Fprintln(stderr, "Run 'portlens conn --help' for usage.")
		return exitcode.InvalidArguments
	}
	return executeCore(ctx, append(preFlags, append([]string{"--connections"}, args...)...), stdout, stderr, stdin)
}

// watchSubcommand handles `portlens watch [port...] [flags]`.
type watchSubcommand struct{}

func (c *watchSubcommand) Name() string      { return "watch" }
func (c *watchSubcommand) Aliases() []string { return nil }
func (c *watchSubcommand) Description() string {
	return "Live-monitor port states with desktop notifications"
}
func (c *watchSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printWatchUsage(stdout)
			return exitcode.Success
		}
	}
	return executeCore(ctx, append(preFlags, append([]string{"--watch"}, args...)...), stdout, stderr, stdin)
}

// findSubcommand handles `portlens find <query> [flags]`.
type findSubcommand struct{}

func (c *findSubcommand) Name() string        { return "find" }
func (c *findSubcommand) Aliases() []string   { return []string{"search"} }
func (c *findSubcommand) Description() string { return "Find ports by process name/command or PID" }
func (c *findSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printFindUsage(stdout)
			return exitcode.Success
		}
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
		if a == "--name" && i+1 < len(args) {
			hasTarget = true
			extraFlags = append(extraFlags, a, args[i+1])
			i++
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
		fmt.Fprintln(stderr, "portlens find: specify process name/query or --pid <pid>")
		fmt.Fprintln(stderr, "Run 'portlens find --help' for usage.")
		return exitcode.InvalidArguments
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
