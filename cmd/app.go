package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/exitcode"
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
	insp := inspector.New(plat)
	if opts != nil && opts.probe {
		insp.EnableProbe = true
	}
	return insp
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
	found := make([]*model.Report, 0, len(ports))
	worst := exitcode.Success
	start := time.Now()

	// Performance optimization: when scanning multiple ports, query the host's
	// active listeners once to build a filter set. This avoids spawning thousands
	// of external processes (e.g. lsof on macOS) or repeatedly re-reading /proc tables.
	var activePorts map[uint16]bool
	if len(ports) > 1 && insp != nil && insp.Platform != nil && insp.Platform.Ports != nil {
		if allListeners, err := insp.Platform.Ports.Listeners(ctx); err == nil {
			activePorts = make(map[uint16]bool, len(allListeners))
			normProto := proto.Normalize()
			for _, l := range allListeners {
				if normProto == "" || l.Protocol.Normalize() == normProto {
					activePorts[l.Port] = true
				}
			}
		}
	}

	// Single port fast path: avoid goroutine synchronization overhead.
	if len(ports) == 1 {
		p := ports[0]
		if activePorts != nil && !activePorts[uint16(p)] {
			if progress != nil {
				progress(1, 1, 0, time.Since(start))
			}
			return nil, exitcode.Success
		}
		report, err := insp.InspectDepth(ctx, p, proto, inspector.DepthFast)
		switch {
		case err == nil:
			if report.Status == "listening" {
				found = append(found, report)
			}
		case errors.Is(err, inspector.ErrPortNotFound):
		default:
			fmt.Fprintf(stderr, "portlens: %v\n", err)
			worst = maxExit(worst, mapError(err))
		}
		if progress != nil {
			progress(1, 1, len(found), time.Since(start))
		}
		return found, worst
	}

	type scanResult struct {
		report *model.Report
		err    error
	}

	type portJob struct {
		index int
		port  int32
	}

	results := make([]scanResult, len(ports))
	var toInspect []portJob
	var idleCount int

	for i, p := range ports {
		if activePorts != nil && !activePorts[uint16(p)] {
			idleCount++
		} else {
			toInspect = append(toInspect, portJob{index: i, port: p})
		}
	}

	var doneCount atomic.Int64
	var foundCount atomic.Int64
	var progressMu sync.Mutex

	reportProgress := func() {
		if progress == nil {
			return
		}
		progressMu.Lock()
		defer progressMu.Unlock()
		progress(int(doneCount.Load()), len(ports), int(foundCount.Load()), time.Since(start))
	}

	if idleCount > 0 {
		doneCount.Add(int64(idleCount))
		reportProgress()
	}

	if len(toInspect) > 0 {
		workers := runtime.NumCPU() * 2
		if workers > 16 {
			workers = 16
		}
		if workers > len(toInspect) {
			workers = len(toInspect)
		}
		if workers < 1 {
			workers = 1
		}
		slog.DebugContext(ctx, "starting concurrent scan", "total_ports", len(ports), "to_inspect", len(toInspect), "workers", workers)

		jobs := make(chan portJob, len(toInspect))
		for _, job := range toInspect {
			jobs <- job
		}
		close(jobs)

		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					if ctx.Err() != nil {
						return
					}
					report, err := insp.InspectDepth(ctx, job.port, proto, inspector.DepthFast)
					results[job.index] = scanResult{report: report, err: err}
					doneCount.Add(1)
					if report != nil && report.Status == "listening" {
						foundCount.Add(1)
					}
					reportProgress()
				}
			}()
		}
		wg.Wait()
	}

	for _, res := range results {
		switch {
		case res.report != nil:
			if res.report.Status == "listening" {
				found = append(found, res.report)
			}
		case res.err == nil || errors.Is(res.err, inspector.ErrPortNotFound):
			// Idle or skipped port
		default:
			fmt.Fprintf(stderr, "portlens: %v\n", res.err)
			worst = maxExit(worst, mapError(res.err))
		}
	}

	return found, worst
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
	insp := newInspector(opts)
	proto := protocolFrom(opts)

	total := len(opts.ports)
	start := time.Now()
	fmt.Fprintf(stdout, "%s\n", scanHeader(opts.ports))

	reporter := newScanProgressReporter(stderr)
	found, worst := scanPorts(ctx, stderr, insp, proto, opts.ports, reporter.Report)
	reporter.Finish()
	elapsed := time.Since(start)

	entries := reportsToEntries(found)
	r := render.New(stdout, !opts.noColor)
	r.List(entries, render.ListOptions{SortBy: opts.sortBy, Filter: opts.filter, OnlyTCP: opts.onlyTCP})
	fmt.Fprintf(stdout, "\nFound %d of %d ports in use in %s.\n", len(found), total, formatElapsed(elapsed))
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

// runPortsJSON inspects every requested port and emits a single JSON array of
// the in-use reports on stdout. Idle ports are omitted, matching scan mode.
// The scan preamble and live progress go to stderr so stdout stays a pure JSON
// payload, ready to pipe into jq or a file.
func runPortsJSON(ctx context.Context, stdout, stderr io.Writer, opts *options) int {
	insp := newInspector(opts)
	proto := protocolFrom(opts)

	total := len(opts.ports)
	start := time.Now()
	fmt.Fprintln(stderr, scanHeader(opts.ports))

	reporter := newScanProgressReporter(stderr)
	found, worst := scanPorts(ctx, stderr, insp, proto, opts.ports, reporter.Report)
	reporter.Finish()
	elapsed := time.Since(start)

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
		fmt.Fprintf(stderr, "portlens: %v\n", err)
		return mapError(err)
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
