package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mishraprayash/Portlens/internal/actions"
	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/inspector"
	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
	"github.com/mishraprayash/Portlens/internal/render"
	"github.com/mishraprayash/Portlens/internal/service"
)

// newInspector builds an inspector honoring the --no-docker escape hatch: when
// set, container detection is disabled entirely.
func newInspector(opts *options) *inspector.Inspector {
	plat := platform.New()
	if opts != nil && opts.noDocker {
		plat.Containers = nil
	}
	var inspOpts []inspector.Option
	if opts != nil && opts.probe {
		inspOpts = append(inspOpts, inspector.WithProbe(true))
	}
	return inspector.New(plat, inspOpts...)
}

// newService builds the list/scan service on top of newInspector so the
// platform wiring (no-docker, probe flags) lives in one place.
func newService(opts *options) *service.PortService {
	return service.New(service.WithInspector(newInspector(opts)))
}

func runListing(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	svc := newService(opts)
	entries, err := svc.List(ctx, opts.onlyTCP)
	if err != nil {
		return fail(stderr, mapError(err), "portlens: %v\n", err)
	}
	if opts.jsonOut {
		_ = render.JSONList(stdout, entries)
		return exitcode.Success
	}
	r := render.NewRenderer(stdout, render.WithColor(!opts.noColor))
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

// inspectDepth picks how much inspection an invocation needs. The fast path
// resolves ownership and the essentials the compact summary shows; the deep
// path additionally computes the process tree, network connections, and
// verbose facts. Interactive mode starts fast and re-inspects on demand for
// the tree/connections keys. --restart needs the ancestor chain to find the
// shell launch command, so it forces full depth.
func inspectDepth(opts *options) inspector.Depth {
	if opts.verbose || opts.tree || opts.connections || opts.restart || opts.jsonOut {
		return inspector.DepthFull
	}
	return inspector.DepthFast
}

// scanMode reports whether a multi-port invocation should use scan mode: scan
// all requested ports, print only the ones in use, show live progress with an
// ETA, and summarize the results at the end. Action and view flags still loop
// port-by-port so their per-port output is preserved.
func scanMode(opts *options) bool {
	return len(opts.ports) > 1 &&
		!opts.jsonOut &&
		!opts.kill && !opts.restart && !opts.open &&
		!opts.tree && !opts.connections
}

// progressInterval is how often the live progress line is refreshed.
const progressInterval = 200 * time.Millisecond

// scanHeader renders the one-line preamble shown before a multi-port scan.
func scanHeader(ports []int32) string {
	return fmt.Sprintf("Scanning %d ports (%s)...", len(ports), formatPorts(ports))
}

// scanPorts is the single shared inspection loop behind scan mode, JSON output,
// and any other multi-port command. It inspects every port, keeps only the
// in-use reports (Status "listening"), treats idle ports as expected rather
// than errors, and calls progress (when non-nil) after each port so callers can
// show a live count, ETA, and found-so-far without duplicating the loop. It
// returns the in-use reports and the most severe exit code seen.
func scanPorts(ctx context.Context, stderr io.Writer, insp *inspector.Inspector, proto model.Protocol, ports []int32, progress func(done, total, found int, elapsed time.Duration)) ([]*model.Report, int) {
	if len(ports) == 0 {
		return nil, exitcode.Success
	}
	svc := service.New(service.WithInspector(insp))
	res, err := svc.Scan(ctx, ports, proto, progress)
	if err != nil {
		return nil, fail(stderr, mapError(err), "portlens: %v\n", err)
	}
	if res.Failed > 0 {
		fmt.Fprintf(stderr, "portlens: warning: %d of %d ports could not be inspected: %v\n",
			res.Failed, len(ports), res.FirstErr)
	}
	return res.Reports, exitcode.Success
}

// scanProgressReporter renders live scan progress to a stream (stderr). On a
// terminal the line is rewritten in place; otherwise it is printed once per
// hundred ports so piped output still shows progress without flooding it.
type scanProgressReporter struct {
	w           io.Writer
	interactive bool
	lastRefresh time.Time
}

func newScanProgressReporter(w io.Writer) *scanProgressReporter {
	return &scanProgressReporter{w: w, interactive: isTerminal(w)}
}

// Report is a drop-in for the scanPorts progress callback.
func (p *scanProgressReporter) Report(done, total, found int, elapsed time.Duration) {
	now := time.Now()
	if p.interactive {
		if now.Sub(p.lastRefresh) >= progressInterval || done == total {
			p.lastRefresh = now
			writeScanProgress(p.w, done, total, elapsed, found, true)
		}
		return
	}
	if done == total || done%100 == 0 {
		writeScanProgress(p.w, done, total, elapsed, found, false)
	}
}

// Finish clears the in-place progress line so the next output starts on a
// fresh line when running on a terminal.
func (p *scanProgressReporter) Finish() {
	if p.interactive {
		fmt.Fprintln(p.w)
	}
}

// runScan inspects every requested port and prints only the in-use ones as a
// table, followed by a summary of how many were found and how long it took.
// Progress goes to stderr so stdout stays clean for the results. The shared
// scanPorts loop and the --log stdout tee keep this free of per-command logic.
func runScan(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	total := len(opts.ports)
	fmt.Fprintf(stdout, "%s\n", scanHeader(opts.ports))

	found, worst, elapsed := scanAll(ctx, stderr, opts)

	entries := reportsToEntries(found)
	r := render.New(stdout, !opts.noColor)
	r.List(entries, render.ListOptions{SortBy: opts.sortBy, Filter: opts.filter, OnlyTCP: opts.onlyTCP})
	fmt.Fprintf(stdout, "\nFound %d of %d ports in use in %s.\n", len(found), total, formatElapsed(elapsed))
	return worst
}

// scanAll runs the shared scan pipeline — progress reporting to progressW,
// then parallel collection — and returns the in-use reports. Callers print
// the header and summary on the stream their output format requires.
func scanAll(ctx context.Context, progressW io.Writer, opts *options) (found []*model.Report, worst int, elapsed time.Duration) {
	insp := newInspector(opts)
	proto := protocolFrom(opts)

	start := time.Now()
	reporter := newScanProgressReporter(progressW)
	found, worst = scanPorts(ctx, progressW, insp, proto, opts.ports, reporter.Report)
	reporter.Finish()
	elapsed = time.Since(start)
	return found, worst, elapsed
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

// runPortsJSON inspects every requested port and emits a single JSON array of
// the in-use reports on stdout. Idle ports are omitted, matching scan mode.
// The scan preamble and live progress go to stderr so stdout stays a pure JSON
// payload, ready to pipe into jq or a file.
func runPortsJSON(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	total := len(opts.ports)
	fmt.Fprintln(stderr, scanHeader(opts.ports))

	found, worst, elapsed := scanAll(ctx, stderr, opts)

	_ = render.JSONReports(stdout, found)
	fmt.Fprintf(stderr, "Found %d of %d ports in use in %s.\n", len(found), total, formatElapsed(elapsed))
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

	report, err := insp.InspectDepth(ctx, port, proto, inspectDepth(opts))
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
		return fail(stderr, mapError(err), "portlens: %v\n", err)
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
			return runInteractive(ctx, insp, mgr, r, report, stdin, stdout, opts)
		}
		renderReport(r, report, opts)
		return exitcode.Success
	}
}

// runAction executes an actions.Manager call and translates its error into a
// fail() message and exit code. special handles action-specific errors with
// tailored messages; otherwise the failure is reported as "<label> failed: %v"
// with the exit code from model.MapExitCode.
func runAction(out io.Writer, label string, fn func() error, special func(err error) (int, string, bool)) int {
	if err := fn(); err != nil {
		if special != nil {
			if code, msg, ok := special(err); ok {
				return fail(out, code, "%s", msg)
			}
		}
		return fail(out, mapError(err), "%s failed: %v\n", label, err)
	}
	return exitcode.Success
}

func runKill(ctx context.Context, mgr *actions.Manager, report *model.Report, opts *options) int {
	return runAction(mgr.Out, "kill", func() error {
		return mgr.Kill(ctx, report, opts.force)
	}, func(err error) (int, string, bool) {
		var still *actions.ErrStillRunning
		if errors.As(err, &still) {
			return exitcode.ProcessActionFailed,
				fmt.Sprintf("Process %d did not exit; use --kill --force to force termination.\n", still.PID), true
		}
		return 0, "", false
	})
}

func runRestart(ctx context.Context, mgr *actions.Manager, report *model.Report) int {
	return runAction(mgr.Out, "restart", func() error {
		return mgr.Restart(ctx, report)
	}, func(err error) (int, string, bool) {
		if errors.Is(err, actions.ErrRestartUnavailable) {
			return exitcode.ProcessActionFailed,
				"Automatic restart is unavailable.\nThe process was not launched from an interactive shell in a way PortLens can reproduce.\n", true
		}
		return 0, "", false
	})
}

func runOpen(ctx context.Context, mgr *actions.Manager, report *model.Report) int {
	return runAction(mgr.Out, "open", func() error {
		return mgr.Open(ctx, report)
	}, nil)
}

func mapError(err error) int {
	return model.MapExitCode(err)
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isTerminalFd(f.Fd())
}
