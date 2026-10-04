package cmd

import (
	"context"
	"flag"
	"io"
	"strconv"
	"strings"

	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/tui"
)

// topSubcommand handles `portlens top` and `portlens tui`.
type topSubcommand struct{}

func (c *topSubcommand) Name() string        { return "top" }
func (c *topSubcommand) Aliases() []string   { return []string{"tui", "dashboard"} }
func (c *topSubcommand) Description() string { return "Full-screen live interactive TUI dashboard" }

func (c *topSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, stdin io.Reader) int {
	allArgs := append(preFlags, args...)
	if wantsHelp(allArgs) {
		printTopUsage(stdout)
		return exitcode.Success
	}

	interval := 2
	onlyTCP := false

	fs := flag.NewFlagSet("portlens top", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&interval, "interval", 2, "")
	fs.IntVar(&interval, "i", 2, "")
	fs.BoolVar(&onlyTCP, "tcp", false, "")

	// Separate recognized flags from stray arguments so typos are reported
	// instead of silently dropped. A bare positive number sets the refresh
	// interval: `portlens top 5`.
	var reordered []string
	for i := 0; i < len(allArgs); i++ {
		a := allArgs[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			if num, err := strconv.Atoi(a); err == nil && num > 0 {
				interval = num
				continue
			}
			return fail(stderr, exitcode.InvalidArguments, "portlens top: unexpected argument %q\nRun 'portlens top --help' for usage.\n", a)
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		switch name {
		case "interval", "i", "tcp":
			reordered = append(reordered, a)
			if name != "tcp" && !strings.Contains(a, "=") && i+1 < len(allArgs) {
				reordered = append(reordered, allArgs[i+1])
				i++
			}
		default:
			return fail(stderr, exitcode.InvalidArguments, "portlens top: unknown flag %q\nRun 'portlens top --help' for usage.\n", a)
		}
	}

	if err := fs.Parse(reordered); err != nil {
		return fail(stderr, exitcode.InvalidArguments, "portlens top: %v\n", err)
	}

	return tui.RunTop(ctx, interval, onlyTCP, stdout, stdin)
}
