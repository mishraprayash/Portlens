package cmd

import (
	"context"
	"flag"
	"fmt"
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
	for _, a := range allArgs {
		if a == "--help" || a == "-h" || a == "help" {
			printTopUsage(stdout)
			return exitcode.Success
		}
	}

	interval := 2
	onlyTCP := false

	fs := flag.NewFlagSet("portlens top", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&interval, "interval", 2, "")
	fs.IntVar(&interval, "i", 2, "")
	fs.BoolVar(&onlyTCP, "tcp", false, "")

	var reordered []string
	for i := 0; i < len(allArgs); i++ {
		a := allArgs[i]
		if a == "--interval" || a == "-i" {
			if i+1 < len(allArgs) {
				reordered = append(reordered, a, allArgs[i+1])
				i++
				continue
			}
		}
		if strings.HasPrefix(a, "--interval=") || strings.HasPrefix(a, "-i=") {
			reordered = append(reordered, a)
			continue
		}
		if a == "--tcp" {
			reordered = append(reordered, a)
			continue
		}
		// If user provides bare number like `portlens top 3`, treat as interval
		if num, err := strconv.Atoi(a); err == nil && num > 0 {
			interval = num
			continue
		}
	}

	if err := fs.Parse(reordered); err != nil {
		fmt.Fprintf(stderr, "portlens top: %v\n", err)
		return exitcode.InvalidArguments
	}

	return tui.RunTop(ctx, interval, onlyTCP, stdout, stdin)
}
