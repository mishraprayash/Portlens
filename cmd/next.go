package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/model"
)

// nextSubcommand handles `portlens next [start-port]`.
type nextSubcommand struct{}

func (n *nextSubcommand) Name() string        { return "next" }
func (n *nextSubcommand) Aliases() []string   { return nil }
func (n *nextSubcommand) Description() string { return "Find the next available listening port" }
func (n *nextSubcommand) Run(ctx context.Context, args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runNext(ctx, args, stdout, stderr)
}

func runNext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("portlens next", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var protocol string
	fs.StringVar(&protocol, "protocol", "tcp", "")

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered.flags); err != nil {
		fmt.Fprintf(stderr, "portlens next: %v\n", err)
		return exitcode.InvalidArguments
	}

	startPort := 3000
	if len(reordered.positional) > 0 {
		p, err := strconv.Atoi(reordered.positional[0])
		if err != nil || p < 1 || p > 65535 {
			fmt.Fprintf(stderr, "portlens next: invalid start port %q (must be 1-65535)\n", reordered.positional[0])
			return exitcode.InvalidArguments
		}
		startPort = p
	}

	proto := model.ProtocolTCP
	network := "tcp"
	if strings.ToLower(protocol) == "udp" {
		proto = model.ProtocolUDP
		network = "udp"
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

	fmt.Fprintf(stderr, "portlens next: no available %s ports found starting from %d\n", network, startPort)
	return exitcode.PortNotFound
}
