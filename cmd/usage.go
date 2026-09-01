package cmd

import (
	"fmt"
	"io"
)

func printUsage(w io.Writer) {
	fmt.Fprint(w, `PortLens — local port intelligence & process management

USAGE
  portlens [port...] [flags]
  portlens <command> [args...] [flags]

With no arguments, PortLens lists the active listening ports on this host.
Specifying one or more ports (or ranges like 3000-3010, groups like @dev) inspects
them, displaying owning process, project framework, git repo, exposure, and actions.

COMMANDS
  list, ls                 List active listening ports (default with no arguments)
  inspect <port...>        Inspect port(s) with process details and exposure risk
  kill <port...>           Gracefully terminate process on port(s) (SIGTERM)
  restart <port...>        Restart the process if launch command is known
  open <port>              Open service in your default web browser
  tree <port>              Display process ancestor and descendant hierarchy
  conn, connections <port> Show active network connections for the process
  watch [port...]          Live-monitor port states with desktop notifications
  find <query>             Find ports by process name/command or PID (--pid)
  next [start]             Find the lowest available/free port (default 3000)
  config                   Manage named port groups (@name)
  completion <shell>       Generate shell autocompletion (bash, zsh, fish)

GENERAL FLAGS
  -v, --verbose     Full detailed report instead of compact summary
  -j, --json        Output pure JSON (array of in-use ports for scans/multi-port)
  -y, --yes         Skip interactive confirmation prompts
  -f, --force       Force termination (SIGKILL) when used with kill
  -p, --probe       Probe HTTP endpoint for status, HTML title, and Server header
  -d, --debug       Enable structured diagnostic logging to stderr
      --protocol <p> Restrict inspection to tcp or udp
      --no-color    Disable colored terminal output
      --no-docker   Disable Docker / container detection
  -h, --help        Show this help
      --version     Print version

LISTING & FILTER FLAGS
      --sort <key>  Sort listing by: port, process, project, runtime
      --filter <s>  Filter listing by substring (matches process, service, project)
      --tcp         Only show TCP listeners

WATCH FLAGS
      --interval <s> Poll interval in seconds (default 1)
      --notify       Desktop notification on port state change

EXIT CODES
  0  success                1  general error       2  invalid arguments
  3  port not found         4  permission denied   5  process action failed
`)
}

func printListUsage(w io.Writer) {
	fmt.Fprint(w, `portlens list — List active listening ports

USAGE
  portlens list [flags]
  portlens ls [flags]

FLAGS
      --sort <key>  Sort by: port, process, project, runtime (default: port)
      --filter <s>  Filter listing by substring
      --tcp         Only show TCP listeners
  -j, --json        Output pure JSON
      --no-color    Disable colored output
      --no-docker   Disable Docker / container detection
  -h, --help        Show this help
`)
}

func printInspectUsage(w io.Writer) {
	fmt.Fprint(w, `portlens inspect — Inspect port(s) with process details and exposure risk

USAGE
  portlens inspect <port...> [flags]

FLAGS
  -v, --verbose     Full detailed report instead of compact summary
  -j, --json        Output pure JSON
  -p, --probe       Probe HTTP endpoint for status, title, and server header
      --protocol <p> Restrict inspection to tcp or udp
      --no-color    Disable colored output
      --no-docker   Disable Docker / container detection
  -h, --help        Show this help
`)
}

func printKillUsage(w io.Writer) {
	fmt.Fprint(w, `portlens kill — Terminate process(es) listening on specified ports

USAGE
  portlens kill <port...> [flags]
  portlens kill --all [flags]

FLAGS
  -f, --force       Force immediate termination (SIGKILL)
  -y, --yes         Skip interactive confirmation prompt
      --all         Terminate all listening processes
  -v, --verbose     Show detailed report before terminating
  -h, --help        Show this help
`)
}

func printRestartUsage(w io.Writer) {
	fmt.Fprint(w, `portlens restart — Restart process if launch command is known

USAGE
  portlens restart <port> [flags]

FLAGS
  -y, --yes         Skip interactive confirmation prompt
  -h, --help        Show this help
`)
}

func printOpenUsage(w io.Writer) {
	fmt.Fprint(w, `portlens open — Open service in your default web browser

USAGE
  portlens open <port> [flags]

FLAGS
  -h, --help        Show this help
`)
}

func printTreeUsage(w io.Writer) {
	fmt.Fprint(w, `portlens tree — Display process ancestor and descendant hierarchy

USAGE
  portlens tree <port> [flags]

FLAGS
      --no-color    Disable colored output
  -h, --help        Show this help
`)
}

func printConnUsage(w io.Writer) {
	fmt.Fprint(w, `portlens conn — Show active network connections for the process

USAGE
  portlens conn <port> [flags]
  portlens connections <port> [flags]

FLAGS
      --no-color    Disable colored output
  -h, --help        Show this help
`)
}

func printWatchUsage(w io.Writer) {
	fmt.Fprint(w, `portlens watch — Live-monitor port states with desktop notifications

USAGE
  portlens watch [port...] [flags]

FLAGS
      --interval <s> Poll interval in seconds (default: 1)
      --notify       Desktop notification on port state change
  -v, --verbose      Detailed report for watched ports
      --no-color     Disable colored output
  -h, --help         Show this help
`)
}

func printFindUsage(w io.Writer) {
	fmt.Fprint(w, `portlens find — Find listening ports by process name or PID

USAGE
  portlens find <name|regex> [flags]
  portlens find --pid <pid> [flags]

FLAGS
      --pid <pid>   Find ports owned by process ID (including children)
  -k, --kill        Terminate matching processes
  -f, --force       Force termination (SIGKILL)
  -y, --yes         Skip interactive confirmation
  -j, --json        Output pure JSON
  -v, --verbose     Full detailed report
  -h, --help        Show this help
`)
}
