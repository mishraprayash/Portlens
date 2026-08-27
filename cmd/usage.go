package cmd

import (
	"fmt"
	"io"
)

func printUsage(w io.Writer) {
	fmt.Fprint(w, `PortLens — local port intelligence & process management

USAGE
  portlens [port] [flags]

  With no port, PortLens lists the interesting listening ports on this host.

COMMANDS / FLAGS
  portlens <port>                 Inspect a port (interactive when on a terminal)
  portlens <port> --tree          Show the complete process hierarchy
  portlens <port> --connections   Show network connections, grouped and summarized
  portlens <port> --json          Machine-readable JSON output
  portlens <port> --kill          Gracefully terminate the owning process (SIGTERM)
  portlens <port> --kill --force  Force termination (SIGKILL)
  portlens <port> --restart       Restart the process if the launch command is known
  portlens <port> --history       Show previously observed activity on this port
  portlens <port> --open          Open the service in your browser
  portlens --version              Print the version
  portlens --help                 Show this help

LISTING FLAGS
  --sort <key>      Sort listing by: port, process, project, runtime
  --filter <text>   Filter listing by a case-insensitive substring
  --tcp             Only show TCP listeners

GENERAL FLAGS
  --protocol <p>    Restrict inspection to tcp or udp
  --yes, -y         Skip interactive confirmations
  --no-color        Disable colored output
  --no-record       Do not record this inspection to local history

EXIT CODES
  0  success                1  general error       2  invalid arguments
  3  port not found         4  permission denied   5  process action failed
`)
}
