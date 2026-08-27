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

func runPort(ctx context.Context, stdout, stderr io.Writer, stdin io.Reader, opts *options) int {
	plat := platform.New()
	insp := inspector.New(plat)

	var proto model.Protocol
	switch opts.protocol {
	case "udp", "udp4", "udp6":
		proto = model.ProtocolUDP
	case "tcp", "tcp4", "tcp6", "":
		proto = model.ProtocolTCP
	}

	if opts.history {
		return runHistory(stdout, stderr, opts)
	}

	report, err := insp.Inspect(ctx, opts.port, proto)
	if err != nil {
		if errors.Is(err, inspector.ErrPortNotFound) {
			r := render.New(stdout, !opts.noColor)
			if opts.jsonOut {
				_ = render.JSON(stdout, report)
			} else {
				r.Report(report)
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
		r.Report(report)
		fmt.Fprintln(stdout)
		return runKill(ctx, mgr, report, opts)
	case opts.restart:
		r.Report(report)
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
		if isTerminal(stdout) && isTerminalReader(stdin) {
			return runInteractive(ctx, plat, mgr, r, report, stdin, stdout, opts)
		}
		r.Report(report)
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

func runHistory(stdout, stderr io.Writer, opts *options) int {
	store, err := history.New()
	if err != nil {
		fmt.Fprintf(stderr, "portlens: unable to open history: %v\n", err)
		return exitcode.GeneralError
	}
	defer store.Close()

	entries, err := store.Query(context.Background(), opts.port, 20)
	if err != nil {
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		return exitcode.GeneralError
	}
	r := render.New(stdout, !opts.noColor)
	r.History(opts.port, entries)
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
