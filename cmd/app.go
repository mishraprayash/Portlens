package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/history"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
	"github.com/portlens/portlens/internal/render"
)

// newInspector builds an inspector honoring the --no-docker escape hatch: when
// set, container detection is disabled entirely.
func newInspector(opts *options) *inspector.Inspector {
	plat := platform.New()
	if opts != nil && opts.noDocker {
		plat.Containers = nil
	}
	return inspector.New(plat)
}

func runListing(stdout, stderr io.Writer, opts *options) int {
	insp := newInspector(opts)
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
	if scanMode(opts) {
		return runScan(ctx, stdout, stderr, opts)
	}

	worst := exitcode.Success
	for _, p := range opts.ports {
		if code := runPort(ctx, stdout, stderr, stdin, opts, p); code > worst {
			worst = code
		}
	}
	return worst
}

// scanMode reports whether a multi-port invocation should use scan mode: scan
// all requested ports, print only the ones in use, show live progress with an
// ETA, and summarize the results at the end. Action and view flags still loop
// port-by-port so their per-port output is preserved.
func scanMode(opts *options) bool {
	return len(opts.ports) > 1 &&
		!opts.jsonOut &&
		!opts.kill && !opts.restart && !opts.open &&
		!opts.tree && !opts.connections && !opts.history
}

// progressInterval is how often the live progress line is refreshed.
const progressInterval = 200 * time.Millisecond

// runScan inspects every requested port, reports only the in-use ones, and
// summarizes the scan. Progress (count, percent, ETA, found so far) goes to
// stderr so stdout stays clean for the results. When --log is given, the full
// report for every in-use port is written to that file.
func runScan(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	insp := newInspector(opts)
	proto := protocolFrom(opts)

	total := len(opts.ports)
	interactive := isTerminal(stderr)
	start := time.Now()

	logFile := (*os.File)(nil)
	if opts.logPath != "" {
		f, err := os.Create(opts.logPath)
		if err != nil {
			fmt.Fprintf(stderr, "portlens: cannot create log file: %v\n", err)
			return exitcode.GeneralError
		}
		defer f.Close()
		logFile = f
		fmt.Fprintln(f, "# PortLens scan log")
		fmt.Fprintf(f, "# Time: %s\n", time.Now().Format(time.RFC3339))
		fmt.Fprintf(f, "# Ports scanned: %d (%s)\n", total, formatPorts(opts.ports))
		fmt.Fprintf(f, "# Protocol: %s\n\n", proto.Normalize())
	}

	fmt.Fprintf(stdout, "Scanning %d ports (%s)...\n", total, formatPorts(opts.ports))

	found := make([]*model.Report, 0, total)
	worst := exitcode.Success
	lastRefresh := time.Time{}
	for i, p := range opts.ports {
		report, err := insp.Inspect(ctx, p, proto)
		switch {
		case err == nil:
			if !opts.noRecord {
				recordHistory(report)
			}
			if report.Status == "listening" {
				found = append(found, report)
			}
		case errors.Is(err, inspector.ErrPortNotFound):
			// Idle ports are expected in a scan, not an error.
		default:
			fmt.Fprintf(stderr, "portlens: %v\n", err)
			worst = maxExit(worst, mapError(err))
		}

		done := i + 1
		now := time.Now()
		if interactive {
			if now.Sub(lastRefresh) >= progressInterval || done == total {
				lastRefresh = now
				writeScanProgress(stderr, done, total, now.Sub(start), len(found), true)
			}
		} else if done == total || done%100 == 0 {
			writeScanProgress(stderr, done, total, now.Sub(start), len(found), false)
		}
	}
	if interactive {
		fmt.Fprintln(stderr)
	}
	elapsed := time.Since(start)

	entries := reportsToEntries(found)
	r := render.New(stdout, !opts.noColor)
	r.List(entries, render.ListOptions{SortBy: opts.sortBy, Filter: opts.filter, OnlyTCP: opts.onlyTCP})
	fmt.Fprintf(stdout, "\nFound %d of %d ports in use in %s.\n", len(found), total, formatElapsed(elapsed))

	if logFile != nil {
		fmt.Fprintf(logFile, "# Ports in use: %d\n", len(found))
		fmt.Fprintf(logFile, "# Elapsed: %s\n\n", formatElapsed(elapsed))
		lw := render.New(logFile, false)
		for _, report := range found {
			fmt.Fprintf(logFile, "===== Port %d =====\n\n", report.Port)
			lw.Report(report)
			fmt.Fprintln(logFile)
		}
		fmt.Fprintf(stdout, "Logged %d result(s) to %s\n", len(found), opts.logPath)
	}

	return worst
}

// writeScanProgress writes a progress line with count, percent, ETA, and the
// number of in-use ports found so far. When cr is true the line is rewritten
// in place (for terminals); otherwise it is printed once per invocation of the
// function (for non-interactive output).
func writeScanProgress(w io.Writer, done, total int, elapsed time.Duration, found int, cr bool) {
	pct := float64(done) / float64(total) * 100
	eta := ""
	if done > 0 {
		per := float64(elapsed) / float64(done)
		eta = " | ETA " + formatElapsed(time.Duration(per*float64(total-done)))
	}
	line := fmt.Sprintf("Scanning %d/%d (%.1f%%) | %d in use%s", done, total, pct, found, eta)
	if cr {
		line = padTo(line, 72)
		fmt.Fprint(w, "\r"+line)
		return
	}
	fmt.Fprintln(w, line)
}

func padTo(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// reportsToEntries flattens reports into listing rows so scan results reuse the
// standard table renderer.
func reportsToEntries(reports []*model.Report) []model.PortEntry {
	entries := make([]model.PortEntry, 0, len(reports))
	for _, r := range reports {
		e := model.PortEntry{
			Port:      r.Port,
			Protocol:  r.Protocol,
			Address:   r.Address,
			Status:    r.Status,
			Service:   r.Service,
			Origin:    r.Origin,
			Container: r.Container,
		}
		if r.Process != nil {
			e.PID = r.Process.PID
			e.Process = r.Process.Name
		}
		if r.Project != nil {
			e.Project = r.Project.Name
			e.Runtime = r.Project.Runtime
		}
		entries = append(entries, e)
	}
	return entries
}

// formatElapsed renders a scan duration compactly: sub-minute scans use
// seconds, longer scans reuse the model duration formatter.
func formatElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return model.FormatDuration(d)
}

// runPortsJSON inspects every port up front so that all reports can be emitted
// as a single JSON array on stdout.
func runPortsJSON(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	insp := newInspector(opts)
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
	insp := newInspector(opts)

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
	mgr := actions.NewManager(insp.Platform, stdout, confirm)

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
			return runInteractive(ctx, insp.Platform, mgr, r, report, stdin, stdout, opts)
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
