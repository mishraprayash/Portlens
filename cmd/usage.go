package cmd

import (
	"fmt"
	"io"
)

func printUsage(w io.Writer) {
	fmt.Fprint(w, `PortLens — local port intelligence & process management

USAGE
  portlens [port ...] [flags]

  With no port, PortLens lists the interesting listening ports on this host.
  Multiple ports, port ranges (e.g. 4000-4010), and named groups (e.g. @dev)
  may be given. A range or several ports start a SCAN: only the ports in use
  are printed, live progress shows a count and ETA, and a summary is printed
  when the scan finishes.

COMMANDS / FLAGS
  portlens <port>...                  Inspect port(s) — compact summary by default
  portlens <port>... --verbose        Full detailed report (-v)
  portlens <port>... --tree           Show the complete process hierarchy
  portlens <port>... --connections    Show network connections, grouped and summarized
  portlens <port>... --json           JSON output (array of the in-use ports for a
                                      range/multiple ports; progress on stderr)
  portlens <port>... --log <file>     Write this command's output to <file> (any
                                      command); plain text, no progress, works
                                      with --json too
  portlens <port>... --kill           Gracefully terminate the owning process(es) (SIGTERM)
  portlens <port>... --kill --force   Force termination (SIGKILL)
  portlens <port>... --restart        Restart the process if the launch command is known
  portlens <port>... --history        Show previously observed activity on each port
  portlens <port>... --open           Open the service in your browser
  portlens --all                      Act on every listening port (e.g. --all --kill)
  portlens --pid <pid>                Find the listening ports owned by a process
  portlens --name <query>             Find ports by process name/command (case-insensitive;
                                      wrap in /.../ to use a regex)
  portlens <port>... --watch          Re-render every --interval seconds until Ctrl-C
  portlens <port>... --watch --notify Notify (macOS/Linux) when a port goes up, down, or changes
  portlens free <port>...             Terminate processes on port(s) to free them immediately
  portlens next [start]               Find the lowest available/free port (default 3000)
  portlens config                     Manage named port groups (@name)
  portlens --version                  Print the version
  portlens --help                     Show this help
LISTING FLAGS
  --sort <key>      Sort listing by: port, process, project, runtime
  --filter <text>   Filter listing by a case-insensitive substring
  --tcp             Only show TCP listeners

WATCH FLAGS
  --interval <secs>  Poll interval for --watch (default 1)
  --notify           Post a desktop notification on state change (requires --watch)

GENERAL FLAGS
  --protocol <p>    Restrict inspection to tcp or udp
  --yes, -y         Skip interactive confirmations
  --no-color        Disable colored output
  --no-record       Do not record this inspection to local history
  --no-docker       Do not query the Docker daemon for container ownership
  --probe, -p       Probe HTTP endpoint for status, HTML title, and Server header
  --debug, -d       Enable structured diagnostic logging to stderr (or PORTLENS_DEBUG=1)
  --log <file>      Capture stdout of this command to a file; plain output,
                    disables interactive mode (cannot be used with --watch)

EXIT CODES
  0  success                1  general error       2  invalid arguments
  3  port not found         4  permission denied   5  process action failed
`)
}
