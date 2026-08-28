package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
)

// freeSubcommand handles `portlens free <port...>`.
type freeSubcommand struct{}

func (f *freeSubcommand) Name() string      { return "free" }
func (f *freeSubcommand) Aliases() []string { return []string{"release"} }
func (f *freeSubcommand) Description() string {
	return "Free port(s) by terminating occupying processes"
}
func (f *freeSubcommand) Run(ctx context.Context, args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	return runFree(ctx, args, stdout, stderr, stdin)
}

func runFree(ctx context.Context, args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	fs := flag.NewFlagSet("portlens free", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var force, yes, noColor bool
	var protocol string
	fs.BoolVar(&force, "force", false, "")
	fs.BoolVar(&force, "f", false, "")
	fs.BoolVar(&yes, "yes", false, "")
	fs.BoolVar(&yes, "y", false, "")
	fs.BoolVar(&noColor, "no-color", false, "")
	fs.StringVar(&protocol, "protocol", "tcp", "")

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered.flags); err != nil {
		fmt.Fprintf(stderr, "portlens free: %v\n", err)
		return exitcode.InvalidArguments
	}

	expanded, err := expandGroups(reordered.positional, configGroupLookup)
	if err != nil {
		fmt.Fprintf(stderr, "portlens free: %v\n", err)
		return exitcode.InvalidArguments
	}

	var ports []int32
	for _, a := range expanded {
		parsed, err := parsePortArg(a)
		if err != nil {
			fmt.Fprintf(stderr, "portlens free: %v\n", err)
			return exitcode.InvalidArguments
		}
		ports = append(ports, parsed...)
	}
	ports = dedupePorts(ports)

	if len(ports) == 0 {
		fmt.Fprintln(stderr, "Usage: portlens free <port...> [--force] [--yes]")
		return exitcode.InvalidArguments
	}

	proto := model.ProtocolTCP
	if strings.ToLower(protocol) == "udp" {
		proto = model.ProtocolUDP
	}

	opts := &options{yes: yes, noColor: noColor}
	insp := newInspector(opts)
	confirm := newConfirm(stdout, stdin, yes)
	mgr := actions.NewManager(insp.Platform, stdout, confirm)

	worst := exitcode.Success
	for _, p := range ports {
		report, err := insp.InspectDepth(ctx, p, proto, inspector.DepthFast)
		if err != nil {
			if errors.Is(err, inspector.ErrPortNotFound) {
				fmt.Fprintf(stdout, "Port %d is already free\n", p)
				continue
			}
			fmt.Fprintf(stderr, "portlens free %d: %v\n", p, err)
			worst = maxExit(worst, mapError(err))
			continue
		}

		targetDesc := fmt.Sprintf("port %d", p)
		if report.Container != nil && report.Container.Name != "" {
			targetDesc = fmt.Sprintf("port %d (container %s)", p, report.Container.Name)
		} else if report.Process != nil && report.Process.Name != "" {
			targetDesc = fmt.Sprintf("port %d (process %s, pid %d)", p, report.Process.Name, report.Process.PID)
		}

		if !yes && confirm != nil {
			ok, err := confirm(fmt.Sprintf("Free %s? [y/N] ", targetDesc))
			if err != nil || !ok {
				fmt.Fprintf(stdout, "Skipped port %d\n", p)
				continue
			}
		}

		if err := mgr.Kill(ctx, report, force); err != nil {
			fmt.Fprintf(stderr, "Failed to free port %d: %v\n", p, err)
			worst = maxExit(worst, exitcode.ProcessActionFailed)
			continue
		}

		// Wait briefly for OS to release the socket
		if insp.Platform != nil && insp.Platform.Ports != nil {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				listeners, err := insp.Platform.Ports.ResolvePort(ctx, uint16(p), proto)
				if err != nil || len(listeners) == 0 {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		fmt.Fprintf(stdout, "Port %d is now free\n", p)
	}

	return worst
}
