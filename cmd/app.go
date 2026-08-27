package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/history"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
	"github.com/portlens/portlens/internal/render"
)

func runListing(stdout, stderr io.Writer, opts *options) int {
	plat := platform.New()
	insp := inspector.New(plat)
	ctx := context.Background()

	entries, err := insp.List(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		return mapError(err)
	}
	if opts.jsonOut {
		_ = render.JSONList(stdout, entries)
		return exitcode.Success
	}
	r := render.New(stdout, !opts.noColor)
	r.List(entries, render.ListOptions{SortBy: opts.sortBy, Filter: opts.filter, OnlyTCP: opts.onlyTCP})
	return exitcode.Success
}

// runPorts dispatches to runPort for each requested port and returns the most
// severe exit code seen. A single port preserves the original behavior,
// including interactive mode.
func runPorts(ctx context.Context, stdout, stderr io.Writer, stdin io.Reader, opts *options) int {
	if opts.jsonOut && len(opts.ports) > 1 {
		return runPortsJSON(ctx, stdout, stderr, opts)
	}

	worst := exitcode.Success
	for _, p := range opts.ports {
		if code := runPort(ctx, stdout, stderr, stdin, opts, p); code > worst {
			worst = code
		}
	}
	return worst
}

// runPortsJSON inspects every port up front so that all reports can be emitted
// as a single JSON array on stdout.
func runPortsJSON(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	plat := platform.New()
	insp := inspector.New(plat)
	proto := protocolFrom(opts)

	reports := make([]*model.Report, 0, len(opts.ports))
	worst := exitcode.Success
	for _, p := range opts.ports {
		report, err := insp.Inspect(ctx, p, proto)
		if err != nil {
			if errors.Is(err, inspector.ErrPortNotFound) {
				reports = append(reports, report)
				worst = maxExit(worst, exitcode.PortNotFound)
				continue
			}
			fmt.Fprintf(stderr, "portlens: %v\n", err)
			worst = maxExit(worst, mapError(err))
			continue
		}
		reports = append(reports, report)
	}
	_ = render.JSONReports(stdout, reports)
	return worst
}

// protocolFrom maps the --protocol flag to a model.Protocol.
func protocolFrom(opts *options) model.Protocol {
	switch opts.protocol {
	case "udp", "udp4", "udp6":
		return model.ProtocolUDP
	default:
		return model.ProtocolTCP
	}
}

// maxExit returns the more severe exit code (higher numeric value wins).
func maxExit(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// renderReport renders a report: the compact Summary by default, or the full
// verbose Report with --verbose.
func renderReport(r *render.Renderer, report *model.Report, opts *options) {
	if opts.verbose {
		r.Report(report)
	} else {
		r.Summary(report)
	}
}

func runPort(ctx context.Context, stdout, stderr io.Writer, stdin io.Reader, opts *options, port int32) int {
	plat := platform.New()
	insp := inspector.New(plat)

	proto := protocolFrom(opts)

	if opts.history {
		return runHistory(stdout, stderr, opts, port)
	}

	report, err := insp.Inspect(ctx, port, proto)
	if err != nil {
		if errors.Is(err, inspector.ErrPortNotFound) {
			r := render.New(stdout, !opts.noColor)
			if opts.jsonOut {
				_ = render.JSON(stdout, report)
			} else {
				renderReport(r, report, opts)
			}
			return exitcode.PortNotFound
		}
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		return mapError(err)
	}

	if !opts.noRecord {
		recordHistory(report)
	}

	r := render.New(stdout, !opts.noColor)

	// Pure output modes first.
	if opts.jsonOut {
		_ = render.JSON(stdout, report)
		return exitcode.Success
	}

	confirm := newConfirm(stdout, stdin, opts.yes)
	mgr := actions.NewManager(plat, stdout, confirm)

	switch {
	case opts.kill:
		renderReport(r, report, opts)
		fmt.Fprintln(stdout)
		return runKill(ctx, mgr, report, opts)
	case opts.restart:
		renderReport(r, report, opts)
		fmt.Fprintln(stdout)
		return runRestart(ctx, mgr, report)
	case opts.open:
		return runOpen(ctx, mgr, report)
	case opts.tree:
		r.Tree(report)
		return exitcode.Success
	case opts.connections:
		r.Connections(report)
		return exitcode.Success
	default:
		if len(opts.ports) == 1 && isTerminal(stdout) && isTerminalReader(stdin) {
			return runInteractive(ctx, plat, mgr, r, report, stdin, stdout, opts)
		}
		renderReport(r, report, opts)
		return exitcode.Success
	}
}

func runKill(ctx context.Context, mgr *actions.Manager, report *model.Report, opts *options) int {
	err := mgr.Kill(ctx, report, opts.force)
	if err != nil {
		var still *actions.ErrStillRunning
		if errors.As(err, &still) {
			fmt.Fprintf(mgr.Out, "Process %d did not exit; use --kill --force to force termination.\n", still.PID)
			return exitcode.ProcessActionFailed
		}
		fmt.Fprintf(mgr.Out, "kill failed: %v\n", err)
		return mapError(err)
	}
	return exitcode.Success
}

func runRestart(ctx context.Context, mgr *actions.Manager, report *model.Report) int {
	if err := mgr.Restart(ctx, report); err != nil {
		if errors.Is(err, actions.ErrRestartUnavailable) {
			fmt.Fprintf(mgr.Out, "Automatic restart is unavailable.\n")
			fmt.Fprintf(mgr.Out, "The process was not launched from an interactive shell in a way PortLens can reproduce.\n")
			return exitcode.ProcessActionFailed
		}
		fmt.Fprintf(mgr.Out, "restart failed: %v\n", err)
		return mapError(err)
	}
	return exitcode.Success
}

func runOpen(ctx context.Context, mgr *actions.Manager, report *model.Report) int {
	if err := mgr.Open(ctx, report); err != nil {
		fmt.Fprintf(mgr.Out, "open failed: %v\n", err)
		return mapError(err)
	}
	return exitcode.Success
}

func runHistory(stdout, stderr io.Writer, opts *options, port int32) int {
	store, err := history.New()
	if err != nil {
		fmt.Fprintf(stderr, "portlens: unable to open history: %v\n", err)
		return exitcode.GeneralError
	}
	defer store.Close()

	entries, err := store.Query(context.Background(), port, 20)
	if err != nil {
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		return exitcode.GeneralError
	}
	r := render.New(stdout, !opts.noColor)
	r.History(port, entries)
	return exitcode.Success
}

func recordHistory(report *model.Report) {
	if report.Process == nil {
		return
	}
	store, err := history.New()
	if err != nil {
		return
	}
	defer store.Close()

	project := ""
	if report.Project != nil {
		project = report.Project.Name
	}
	_ = store.Record(context.Background(), model.HistoryEntry{
		Port:       report.Port,
		ObservedAt: time.Now(),
		PID:        report.Process.PID,
		Process:    report.Process.Name,
		Project:    project,
		Command:    report.Process.Command,
		Address:    report.Address,
		Status:     "seen",
	})
}

func mapError(err error) int {
	switch {
	case errors.Is(err, platform.ErrPermissionDenied):
		return exitcode.PermissionDenied
	case errors.Is(err, platform.ErrProcessNotFound):
		return exitcode.PortNotFound
	default:
		return exitcode.GeneralError
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isTerminalFd(f.Fd())
}
