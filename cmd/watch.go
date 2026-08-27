package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
	"github.com/portlens/portlens/internal/render"
)

// watchSnap captures the observed state of the watched targets at one point in
// time. Keys are human-readable target labels; values describe their state.
type watchSnap map[string]string

type watchChange struct {
	kind   string // "up" | "down" | "changed"
	target string
	detail string
}

// runWatch re-renders the requested ports (or the full listing when no ports
// are given) every interval seconds, optionally posting a desktop notification
// whenever a target's state changes. It exits cleanly on Ctrl-C or SIGTERM.
func runWatch(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	interval := opts.interval
	if interval <= 0 {
		interval = 1
	}
	interactive := isTerminal(stdout)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	var prev watchSnap
	first := true
	for {
		if interactive {
			clearScreen(stdout)
		}
		fmt.Fprintf(stdout, "%s — updated %s\n\n", watchTargetLabel(opts), time.Now().Format("15:04:05"))

		cur, err := renderWatchTick(ctx, stdout, stderr, opts)
		if err != nil {
			// Transient failure: keep the previous snapshot so a single bad
			// tick does not report every target as down.
			first = false
			select {
			case <-sig:
				if interactive {
					fmt.Fprintln(stdout)
				}
				return exitcode.Success
			case <-ticker.C:
			}
			continue
		}

		// The first tick only establishes a baseline; notifications are posted
		// when a later tick detects an actual change.
		if opts.notify && !first {
			notifyChanges(ctx, diffWatch(prev, cur))
		}
		prev = cur
		first = false

		select {
		case <-sig:
			if interactive {
				fmt.Fprintln(stdout)
			}
			return exitcode.Success
		case <-ticker.C:
		}
	}
}

// renderWatchTick inspects the current state, renders it to stdout, and
// returns a snapshot for change detection. A fresh inspector is created each
// tick so long-running sessions never see stale process or socket data. On an
// unrecoverable inspection error it reports to stderr and returns a non-nil
// error so the caller can skip change detection for that tick.
func renderWatchTick(ctx context.Context, stdout, stderr io.Writer, opts *options) (watchSnap, error) {
	plat := platform.New()
	insp := inspector.New(plat)
	r := render.New(stdout, !opts.noColor)
	s := watchSnap{}

	if len(opts.ports) == 0 {
		entries, err := insp.List(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "portlens: %v\n", err)
			return nil, err
		}
		r.List(entries, render.ListOptions{SortBy: opts.sortBy, Filter: opts.filter, OnlyTCP: opts.onlyTCP})
		for _, e := range entries {
			s[watchListKey(e)] = fmt.Sprintf("%d:%s", e.PID, e.Process)
		}
		return s, nil
	}

	proto := protocolFrom(opts)
	for _, p := range opts.ports {
		report, err := insp.Inspect(ctx, p, proto)
		if err != nil {
			if errors.Is(err, inspector.ErrPortNotFound) {
				r.Report(&model.Report{Port: p, Protocol: proto, Status: "not_listening"})
				s[watchPortKey(p)] = "down"
				continue
			}
			fmt.Fprintf(stderr, "portlens: %v\n", err)
			s[watchPortKey(p)] = "error"
			continue
		}
		renderReport(r, report, opts)
		if report.Process != nil {
			s[watchPortKey(p)] = fmt.Sprintf("up:%d:%s", report.Process.PID, report.Process.Name)
		} else {
			s[watchPortKey(p)] = "up:owner-unknown"
		}
	}
	return s, nil
}

// diffWatch compares two snapshots and returns the changes in stable order.
func diffWatch(prev, cur watchSnap) []watchChange {
	keys := map[string]bool{}
	for k := range prev {
		keys[k] = true
	}
	for k := range cur {
		keys[k] = true
	}
	var out []watchChange
	for k := range keys {
		pv, pok := prev[k]
		cv, cok := cur[k]
		switch {
		case cok && !pok:
			out = append(out, watchChange{kind: "up", target: k, detail: cv})
		case !cok && pok:
			out = append(out, watchChange{kind: "down", target: k})
		case cok && pok && pv != cv:
			switch {
			case isDown(pv) && !isDown(cv):
				out = append(out, watchChange{kind: "up", target: k, detail: cv})
			case !isDown(pv) && isDown(cv):
				out = append(out, watchChange{kind: "down", target: k})
			default:
				out = append(out, watchChange{kind: "changed", target: k, detail: cv})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].target < out[j].target })
	return out
}

// isDown reports whether a snapshot value describes a target that is not
// listening. Port snapshots use "down"; list entries simply disappear, so this
// is only meaningful for watched ports.
func isDown(v string) bool {
	return strings.HasPrefix(v, "down")
}

func notifyChanges(ctx context.Context, changes []watchChange) {
	for _, c := range changes {
		title := "PortLens"
		switch c.kind {
		case "up":
			title = "PortLens: up"
		case "down":
			title = "PortLens: down"
		}
		_ = platform.Notify(ctx, title, changeText(c))
	}
}

func changeText(c watchChange) string {
	switch c.kind {
	case "up":
		return fmt.Sprintf("%s is now listening (%s)", c.target, c.detail)
	case "down":
		return fmt.Sprintf("%s is no longer listening", c.target)
	default:
		return fmt.Sprintf("%s changed (%s)", c.target, c.detail)
	}
}

func watchTargetLabel(opts *options) string {
	if len(opts.ports) == 0 {
		return "All listening ports"
	}
	return "Ports " + formatPorts(opts.ports)
}

func watchPortKey(port int32) string { return fmt.Sprintf("Port %d", port) }
func watchListKey(e model.PortEntry) string {
	return fmt.Sprintf("Port %d (%s)", e.Port, e.Protocol.Normalize())
}

func clearScreen(w io.Writer) {
	fmt.Fprint(w, "\x1b[2J\x1b[H")
}
