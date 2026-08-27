// Package cmd implements the PortLens command-line interface.
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/version"
)

var errHelp = errors.New("help requested")

type options struct {
	port     int32
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

	yes      bool
	noRecord bool
	noColor  bool
	help     bool
	showVer  bool
}

// Execute runs the CLI and returns a process exit code.
func Execute(args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	opts, err := parseArgs(args)
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

	if opts.port == 0 {
		return runListing(stdout, stderr, opts)
	}
	return runPort(context.Background(), stdout, stderr, stdin, opts)
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
	fs.BoolVar(&opts.onlyTCP, "tcp", false, "")
	fs.BoolVar(&opts.help, "help", false, "")
	fs.BoolVar(&opts.help, "h", false, "")
	fs.BoolVar(&opts.showVer, "version", false, "")
	fs.BoolVar(&opts.showVer, "v", false, "")
	fs.StringVar(&opts.protocol, "protocol", "", "")
	fs.StringVar(&opts.sortBy, "sort", "port", "")
	fs.StringVar(&opts.filter, "filter", "", "")

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered.flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, errHelp
		}
		return nil, err
	}

	rest := reordered.positional
	if len(rest) > 1 {
		return nil, fmt.Errorf("too many arguments: %q", strings.Join(rest, " "))
	}
	if len(rest) == 1 {
		p, err := strconv.Atoi(rest[0])
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid port %q (must be 1-65535)", rest[0])
		}
		opts.port = int32(p)
	}

	switch strings.ToLower(opts.protocol) {
	case "", "tcp", "tcp4", "tcp6":
	case "udp", "udp4", "udp6":
	default:
		return nil, fmt.Errorf("invalid --protocol %q (must be tcp or udp)", opts.protocol)
	}

	if opts.force && !opts.kill {
		return nil, fmt.Errorf("--force requires --kill")
	}

	return opts, nil
}

// reorderArgs separates flags from positional arguments so that flags may
// appear before or after the port (e.g. "portlens 3000 --tree").
type argSplit struct {
	flags      []string
	positional []string
}

var valueFlags = map[string]bool{
	"protocol": true, "sort": true, "filter": true,
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
