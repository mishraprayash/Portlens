package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/model"
)

// nextSubcommand handles `portlens next [start-port]`.
type nextSubcommand struct{}

func (n *nextSubcommand) Name() string        { return "next" }
func (n *nextSubcommand) Aliases() []string   { return []string{"free"} }
func (n *nextSubcommand) Description() string { return "Find the lowest available/free port" }
func (n *nextSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, _ io.Reader) int {
	if wantsHelp(args) {
		printNextUsage(stdout)
		return exitcode.Success
	}
	return runNext(ctx, append(preFlags, args...), stdout, stderr)
}

func printNextUsage(w io.Writer) {
	fmt.Fprint(w, `portlens next — Find the lowest available/free port

USAGE
  portlens next [start-port] [flags]
  portlens free [start-port] [flags]

FLAGS
      --protocol <p> Protocol to check: tcp or udp (default: tcp)
  -h, --help         Show this help
`)
}

func runNext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("portlens next", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var protocol string
	fs.StringVar(&protocol, "protocol", "tcp", "")

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered.flags); err != nil {
		return fail(stderr, exitcode.InvalidArguments, "portlens next: %v\n", err)
	}

	startPort := 3000
	if len(reordered.positional) > 0 {
		p, err := strconv.Atoi(reordered.positional[0])
		if err != nil || p < 1 || p > 65535 {
			return fail(stderr, exitcode.InvalidArguments, "portlens next: invalid start port %q (must be 1-65535)\n", reordered.positional[0])
		}
		startPort = p
	}

	proto := model.ProtocolTCP
	network := "tcp"
	switch strings.ToLower(protocol) {
	case "", "tcp", "tcp4", "tcp6":
	case "udp", "udp4", "udp6":
		proto = model.ProtocolUDP
		network = "udp"
	default:
		return fail(stderr, exitcode.InvalidArguments, "portlens next: invalid --protocol %q (must be tcp or udp)\n", protocol)
	}

	insp := newInspector(&options{})
	inUse := make(map[uint16]bool)
	if insp.Platform != nil && insp.Platform.Ports != nil {
		if listeners, err := insp.Platform.Ports.Listeners(ctx); err == nil {
			normProto := proto.Normalize()
			for _, l := range listeners {
				if normProto == "" || l.Protocol.Normalize() == normProto {
					inUse[l.Port] = true
				}
			}
		}
	}

	for p := startPort; p <= 65535; p++ {
		if ctx.Err() != nil {
			// User interrupted the search: stop instead of probing up to 65535.
			return exitcode.Success
		}
		if inUse[uint16(p)] {
			continue
		}

		// Double-check by attempting a local bind
		if network == "tcp" {
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
			if err == nil {
				_ = ln.Close()
				fmt.Fprintln(stdout, p)
				return exitcode.Success
			}
		} else {
			pc, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", p))
			if err == nil {
				_ = pc.Close()
				fmt.Fprintln(stdout, p)
				return exitcode.Success
			}
		}
	}

	return fail(stderr, exitcode.PortNotFound, "portlens next: no available %s ports found starting from %d\n", network, startPort)
}
