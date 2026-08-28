// Package cmd implements the PortLens command-line interface.
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/portlens/portlens/internal/config"
	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/version"
)

var errHelp = errors.New("help requested")

// maxPortsPerInvocation bounds how many ports a single range may expand to. The
// full port space is allowed because multi-port invocations use scan mode: they
// print only in-use ports, show live progress with an ETA, and can be aborted
// with Ctrl-C. Ports must still fall within 1-65535, so a typo like "1-99999"
// is rejected before any work is started.
const maxPortsPerInvocation = 65535

type options struct {
	ports    []int32
	protocol string

	tree        bool
	connections bool
	jsonOut     bool
	kill        bool
	force       bool
	restart     bool
	open        bool
	history     bool

	sortBy  string
	filter  string
	onlyTCP bool

	all  bool
	pid  int
	name string

	logPath string

	watch    bool
	interval int
	notify   bool

	verbose bool
	debug   bool
	probe   bool

	yes      bool
	noRecord bool
	noColor  bool
	noDocker bool
	help     bool
	showVer  bool
}

// Execute runs the CLI and returns a process exit code.
func Execute(args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	debug := os.Getenv("PORTLENS_DEBUG") != ""
	for _, a := range args {
		if a == "--debug" || a == "-debug" || a == "-d" {
			debug = true
			break
		}
	}
	initLogger(debug, stderr)

	for _, a := range args {
		if a == "--_complete_ports" {
			return runCompletePorts(stdout)
		}
	}

	reg := defaultSubcommandRegistry()
	if subCmd, subArgs, preFlags := extractSubcommand(args, reg); subCmd != nil {
		for i := 0; i < len(preFlags); i++ {
			f := preFlags[i]
			if (f == "--log" || f == "-log") && i+1 < len(preFlags) {
				w, file, err := teeLog(stdout, preFlags[i+1])
				if err == nil {
					defer file.Close()
					stdout = w
				}
				i++
			}
		}
		return subCmd.Run(context.Background(), subArgs, stdout, stderr, stdin)
	}

	expanded, err := expandGroups(args, configGroupLookup)
	if err != nil {
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		fmt.Fprintf(stderr, "Manage groups with: portlens config add <name> <port> [port ...]\n")
		return exitcode.InvalidArguments
	}

	opts, err := parseArgs(expanded)
	if err == errHelp {
		printUsage(stdout)
		return exitcode.Success
	}
	if err != nil {
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		fmt.Fprintf(stderr, "Run 'portlens --help' for usage.\n")
		return exitcode.InvalidArguments
	}
	if opts.help {
		printUsage(stdout)
		return exitcode.Success
	}
	if opts.showVer {
		fmt.Fprintf(stdout, "portlens %s\n", version.Version)
		return exitcode.Success
	}

	// --log tees stdout to a file, so any command's output can be captured with
	// one mechanism instead of per-command plumbing. Progress and diagnostics
	// (stderr) are intentionally not captured, and wrapping stdout disables
	// color and interactive mode so the log stays plain and complete.
	if opts.logPath != "" {
		w, f, err := teeLog(stdout, opts.logPath)
		if err != nil {
			fmt.Fprintf(stderr, "portlens: cannot create log file: %v\n", err)
			return exitcode.GeneralError
		}
		defer f.Close()
		stdout = w
		fmt.Fprintf(stderr, "portlens: logging output to %s\n", opts.logPath)
	}

	// Dynamic port sources resolve at runtime: --all, --pid, and --name.
	if opts.all || opts.pid > 0 || opts.name != "" {
		if code := resolveDynamicPorts(stdout, stderr, opts); code != exitcode.Success {
			return code
		}
	}

	if opts.watch {
		return runWatch(context.Background(), stdout, stderr, opts)
	}

	if len(opts.ports) == 0 {
		return runListing(stdout, stderr, opts)
	}
	return runPorts(context.Background(), stdout, stderr, stdin, opts)
}

func parseArgs(args []string) (*options, error) {
	opts := &options{}
	fs := flag.NewFlagSet("portlens", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.BoolVar(&opts.tree, "tree", false, "")
	fs.BoolVar(&opts.tree, "t", false, "")
	fs.BoolVar(&opts.connections, "connections", false, "")
	fs.BoolVar(&opts.connections, "n", false, "")
	fs.BoolVar(&opts.jsonOut, "json", false, "")
	fs.BoolVar(&opts.kill, "kill", false, "")
	fs.BoolVar(&opts.kill, "k", false, "")
	fs.BoolVar(&opts.force, "force", false, "")
	fs.BoolVar(&opts.force, "f", false, "")
	fs.BoolVar(&opts.restart, "restart", false, "")
	fs.BoolVar(&opts.restart, "r", false, "")
	fs.BoolVar(&opts.open, "open", false, "")
	fs.BoolVar(&opts.open, "o", false, "")
	fs.BoolVar(&opts.history, "history", false, "")
	fs.BoolVar(&opts.yes, "yes", false, "")
	fs.BoolVar(&opts.yes, "y", false, "")
	fs.BoolVar(&opts.noRecord, "no-record", false, "")
	fs.BoolVar(&opts.noColor, "no-color", false, "")
	fs.BoolVar(&opts.noDocker, "no-docker", false, "")
	fs.BoolVar(&opts.onlyTCP, "tcp", false, "")
	fs.BoolVar(&opts.all, "all", false, "")
	fs.BoolVar(&opts.watch, "watch", false, "")
	fs.BoolVar(&opts.watch, "w", false, "")
	fs.BoolVar(&opts.notify, "notify", false, "")
	fs.BoolVar(&opts.verbose, "verbose", false, "")
	fs.BoolVar(&opts.verbose, "v", false, "")
	fs.BoolVar(&opts.debug, "debug", false, "")
	fs.BoolVar(&opts.debug, "d", false, "")
	fs.BoolVar(&opts.probe, "probe", false, "")
	fs.BoolVar(&opts.probe, "p", false, "")
	fs.IntVar(&opts.interval, "interval", 0, "")
	fs.IntVar(&opts.pid, "pid", 0, "")
	fs.StringVar(&opts.name, "name", "", "")
	fs.BoolVar(&opts.help, "help", false, "")
	fs.BoolVar(&opts.help, "h", false, "")
	fs.BoolVar(&opts.showVer, "version", false, "")
	fs.StringVar(&opts.protocol, "protocol", "", "")
	fs.StringVar(&opts.sortBy, "sort", "port", "")
	fs.StringVar(&opts.filter, "filter", "", "")
	fs.StringVar(&opts.logPath, "log", "", "")

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered.flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, errHelp
		}
		return nil, err
	}

	// Record which flags were explicitly supplied so zero values can be
	// distinguished from "not provided" during validation.
	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })

	if provided["pid"] && opts.pid <= 0 {
		return nil, fmt.Errorf("--pid must be a positive process ID")
	}
	if provided["name"] && strings.TrimSpace(opts.name) == "" {
		return nil, fmt.Errorf("--name must not be empty")
	}
	if provided["interval"] && opts.interval <= 0 {
		return nil, fmt.Errorf("--interval must be a positive number of seconds")
	}
	if opts.interval > 0 && !opts.watch {
		return nil, fmt.Errorf("--interval requires --watch")
	}
	if opts.notify && !opts.watch {
		return nil, fmt.Errorf("--notify requires --watch")
	}

	rest := reordered.positional
	if len(rest) > 0 && (opts.all || opts.pid > 0 || opts.name != "") {
		return nil, fmt.Errorf("cannot combine explicit ports with --all, --pid, or --name")
	}
	for _, arg := range rest {
		if strings.HasPrefix(arg, "@") {
			return nil, fmt.Errorf("unknown port group %q", arg)
		}
		ports, err := parsePortArg(arg)
		if err != nil {
			return nil, err
		}
		opts.ports = append(opts.ports, ports...)
	}
	opts.ports = dedupePorts(opts.ports)

	switch strings.ToLower(opts.protocol) {
	case "", "tcp", "tcp4", "tcp6":
	case "udp", "udp4", "udp6":
	default:
		return nil, fmt.Errorf("invalid --protocol %q (must be tcp or udp)", opts.protocol)
	}

	if opts.force && !opts.kill {
		return nil, fmt.Errorf("--force requires --kill")
	}

	if opts.logPath != "" && opts.watch {
		return nil, fmt.Errorf("--log cannot be used with --watch")
	}

	return opts, nil
}

// parsePortArg parses a single port or a port range: "3000", "3000-3010", or
// "3000:3010". Ranges are bounded by maxPortsPerInvocation.
func parsePortArg(s string) ([]int32, error) {
	loStr, hiStr := s, s
	if i := strings.IndexAny(s, "-:"); i >= 0 {
		loStr, hiStr = s[:i], s[i+1:]
	}
	lo, errLo := strconv.Atoi(loStr)
	hi, errHi := strconv.Atoi(hiStr)
	if errLo != nil || errHi != nil || lo < 1 || hi > 65535 || lo > hi {
		return nil, fmt.Errorf("invalid port %q (must be 1-65535 or a range like 3000-3010)", s)
	}
	n := hi - lo + 1
	if n > maxPortsPerInvocation {
		return nil, fmt.Errorf("port range %q expands to %d ports (max %d)", s, n, maxPortsPerInvocation)
	}
	out := make([]int32, 0, n)
	for p := lo; p <= hi; p++ {
		out = append(out, int32(p))
	}
	return out, nil
}

// reorderArgs separates flags from positional arguments so that flags may
// appear before or after the port (e.g. "portlens 3000 --tree").
type argSplit struct {
	flags      []string
	positional []string
}

var valueFlags = map[string]bool{
	"protocol": true, "sort": true, "filter": true, "name": true, "pid": true, "interval": true, "log": true,
}

func reorderArgs(args []string) argSplit {
	var out argSplit
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			out.positional = append(out.positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			out.flags = append(out.flags, a)
			// If this flag expects a value and the value is provided as a
			// separate argument, pull it along.
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			}
			if valueFlags[name] && !strings.Contains(a, "=") && i+1 < len(args) {
				out.flags = append(out.flags, args[i+1])
				i++
			}
			continue
		}
		out.positional = append(out.positional, a)
	}
	return out
}

// dedupePorts removes duplicate ports while preserving first-seen order.
func dedupePorts(ports []int32) []int32 {
	seen := make(map[int32]bool, len(ports))
	out := make([]int32, 0, len(ports))
	for _, p := range ports {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// groupLookup resolves a named port group to its ports.
type groupLookup func(name string) ([]int32, error)

// configGroupLookup resolves groups from the user configuration file.
func configGroupLookup(name string) ([]int32, error) {
	c, err := config.Load()
	if err != nil {
		return nil, err
	}
	ports, ok := c.Ports(name)
	if !ok {
		return nil, fmt.Errorf("group %q not found", name)
	}
	return ports, nil
}

// expandGroups replaces positional `@group` references with the group's ports
// while leaving flags and their values untouched. A non-existent group is an
// error.
func expandGroups(args []string, lookup groupLookup) ([]string, error) {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "@") && len(a) > 1 {
			ports, err := lookup(a[1:])
			if err != nil {
				return nil, fmt.Errorf("unknown port group %q", a)
			}
			for _, p := range ports {
				out = append(out, strconv.Itoa(int(p)))
			}
			continue
		}
		out = append(out, a)
		// Pull value flags along so their values are not mistaken for groups.
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if valueFlags[name] && !strings.Contains(a, "=") && i+1 < len(args) {
			out = append(out, args[i+1])
			i++
		}
	}
	return out, nil
}

// resolveDynamicPorts fills opts.ports from --all, --pid, or --name. It returns
// exitcode.Success on success or a nonzero exit code on failure.
func resolveDynamicPorts(stdout, stderr io.Writer, opts *options) int {
	if len(opts.ports) > 0 {
		fmt.Fprintf(stderr, "portlens: cannot combine explicit ports with --all/--pid/--name\n")
		return exitcode.InvalidArguments
	}
	insp := newInspector(opts)
	ctx := context.Background()
	var entries []model.PortEntry
	var err error
	switch {
	case opts.pid > 0:
		entries, err = insp.SearchByPID(ctx, int32(opts.pid))
	case opts.name != "":
		entries, err = insp.SearchByName(ctx, opts.name)
	default:
		entries, err = insp.List(ctx)
	}
	if err != nil {
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		return mapError(err)
	}
	if len(entries) == 0 {
		fmt.Fprintf(stderr, "portlens: no %s\n", describeTarget(opts))
		return exitcode.PortNotFound
	}
	for _, e := range entries {
		opts.ports = append(opts.ports, e.Port)
	}
	opts.ports = dedupePorts(opts.ports)
	return exitcode.Success
}

// describeTarget returns a human-readable description of the port source for
// error messages and watch-mode headers.
func describeTarget(opts *options) string {
	switch {
	case opts.pid > 0:
		return fmt.Sprintf("listening port owned by process %d", opts.pid)
	case opts.name != "":
		return fmt.Sprintf("listening port owned by a process matching %q", opts.name)
	case opts.all:
		return "listening port"
	default:
		return "matching listening port"
	}
}

// initLogger configures the default slog logger for diagnostic tracing.
func initLogger(debug bool, w io.Writer) {
	if debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1})))
	}
}
