package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mishraprayash/Portlens/internal/config"
	"github.com/mishraprayash/Portlens/internal/exitcode"
)

// runConfig dispatches the `portlens config` subcommand, which manages named
// port groups used as `portlens @<group>`.
func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printConfigUsage(stdout)
		return exitcode.Success
	}
	switch args[0] {
	case "list", "ls":
		return configList(stdout, stderr)
	case "show":
		return configShow(args[1:], stdout, stderr)
	case "add":
		return configAdd(args[1:], stdout, stderr)
	case "remove", "rm":
		return configRemove(args[1:], stdout, stderr)
	case "path":
		fmt.Fprintln(stdout, config.Path())
		return exitcode.Success
	case "help", "--help", "-h":
		printConfigUsage(stdout)
		return exitcode.Success
	default:
		fmt.Fprintf(stderr, "portlens config: unknown subcommand %q\n", args[0])
		printConfigUsage(stderr)
		return exitcode.InvalidArguments
	}
}

// loadConfig reads the group config, reporting the standard
// "portlens config: %v" error (exit 1) when the file cannot be read.
// On failure it returns a nil config and the exit code to return.
func loadConfig(stderr io.Writer) (*config.Config, int) {
	c, err := config.Load()
	if err != nil {
		return nil, fail(stderr, exitcode.GeneralError, "portlens config: %v\n", err)
	}
	return c, exitcode.Success
}

// saveConfig writes the group config back with the standard error handling.
func saveConfig(c *config.Config, stderr io.Writer) int {
	if err := c.Save(); err != nil {
		return fail(stderr, exitcode.GeneralError, "portlens config: %v\n", err)
	}
	return exitcode.Success
}

func configList(stdout, stderr io.Writer) int {
	c, code := loadConfig(stderr)
	if c == nil {
		return code
	}
	names := c.GroupNames()
	if len(names) == 0 {
		fmt.Fprintln(stdout, "No port groups configured.")
		fmt.Fprintln(stdout, "Add one with: portlens config add <name> <port> [port ...]")
		return exitcode.Success
	}
	fmt.Fprintf(stdout, "Port groups (%s):\n", config.Path())
	for _, n := range names {
		fmt.Fprintf(stdout, "  @%-12s %s\n", n, formatPorts(c.Groups[n]))
	}
	return exitcode.Success
}

func configShow(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return fail(stderr, exitcode.InvalidArguments, "usage: portlens config show <name>\n")
	}
	c, code := loadConfig(stderr)
	if c == nil {
		return code
	}
	ports, ok := c.Ports(args[0])
	if !ok {
		return fail(stderr, exitcode.InvalidArguments, "portlens config: group %q not found\n", args[0])
	}
	fmt.Fprintln(stdout, formatPorts(ports))
	return exitcode.Success
}

func configAdd(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return fail(stderr, exitcode.InvalidArguments, "usage: portlens config add <name> <port|range> [port|range ...]\n")
	}
	name := args[0]
	var ports []int32
	for _, a := range args[1:] {
		p, err := parsePortArg(a)
		if err != nil {
			return fail(stderr, exitcode.InvalidArguments, "portlens config: %v\n", err)
		}
		ports = append(ports, p...)
	}
	ports = dedupePorts(ports)

	c, code := loadConfig(stderr)
	if c == nil {
		return code
	}
	if err := c.SetGroup(name, ports); err != nil {
		return fail(stderr, exitcode.InvalidArguments, "portlens config: %v\n", err)
	}
	if code := saveConfig(c, stderr); code != exitcode.Success {
		return code
	}
	fmt.Fprintf(stdout, "Saved group @%s: %s\n", name, formatPorts(ports))
	return exitcode.Success
}

func configRemove(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return fail(stderr, exitcode.InvalidArguments, "usage: portlens config remove <name>\n")
	}
	c, code := loadConfig(stderr)
	if c == nil {
		return code
	}
	if !c.RemoveGroup(args[0]) {
		return fail(stderr, exitcode.InvalidArguments, "portlens config: group %q not found\n", args[0])
	}
	if code := saveConfig(c, stderr); code != exitcode.Success {
		return code
	}
	fmt.Fprintf(stdout, "Removed group @%s\n", args[0])
	return exitcode.Success
}

// formatPorts renders a port list compactly with consecutive ports collapsed
// into ranges (e.g. "3000, 3002, 4000-4004").
func formatPorts(ports []int32) string {
	sorted := append([]int32(nil), ports...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var sb strings.Builder
	for i := 0; i < len(sorted); {
		j := i
		for j+1 < len(sorted) && sorted[j+1] == sorted[j]+1 {
			j++
		}
		if i > 0 {
			sb.WriteString(", ")
		}
		if j > i {
			fmt.Fprintf(&sb, "%d-%d", sorted[i], sorted[j])
		} else {
			fmt.Fprintf(&sb, "%d", sorted[i])
		}
		i = j + 1
	}
	return sb.String()
}

func printConfigUsage(w io.Writer) {
	fmt.Fprintf(w, `Manage named port groups (usable as: portlens @<group>)

USAGE
  portlens config list                     List all groups
  portlens config show <name>              Show one group's ports
  portlens config add <name> <ports...>    Create or replace a group
  portlens config remove <name>            Delete a group
  portlens config path                     Print the config file path
  portlens config help                     Show this help

<ports...> accepts single ports and ranges, e.g. "3000 4000-4010".
Groups are stored in %s.
`, config.Path())
}
