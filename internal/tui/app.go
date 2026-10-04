package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mishraprayash/Portlens/internal/actions"
	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/inspector"
	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
	"github.com/mishraprayash/Portlens/internal/service"
	"golang.org/x/term"
)

// AppConfig configures the TUI application.
type AppConfig struct {
	Service  *service.PortService
	Platform *platform.Platform
	Interval time.Duration
	OnlyTCP  bool
	Stdout   io.Writer
	Stdin    io.Reader
}

// App runs the full-screen interactive TUI dashboard.
type App struct {
	cfg     AppConfig
	model   *Model
	actions *actions.Manager

	mu sync.Mutex

	// Background list/report work is single-flight: a slow run coalesces any
	// requests that arrive while it is busy into exactly one follow-up,
	// instead of stacking concurrent port scans on every tick or keystroke.
	refresh    singleFlight
	report     singleFlight
	reportWant atomic.Pointer[reportTarget]
}

// reportTarget is the port/protocol the in-flight report fetch should inspect;
// it is re-read on every follow-up run so a selection change made while a
// fetch is busy is served by the queued run.
type reportTarget struct {
	port  int32
	proto model.Protocol
}

// NewApp creates a new TUI App instance.
func NewApp(cfg AppConfig) *App {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.Platform == nil {
		cfg.Platform = platform.New()
	}
	if cfg.Service == nil {
		cfg.Service = service.New(service.WithInspector(inspector.New(cfg.Platform)))
	}

	a := &App{
		cfg:   cfg,
		model: NewModel(cfg.OnlyTCP),
	}
	// Action output (browser-open warnings, clipboard notes) surfaces in the
	// status bar instead of being discarded.
	a.actions = actions.NewManager(cfg.Platform, statusWriter{a: a}, nil)
	return a
}

// statusWriter joins action-manager output into a single status-bar line.
type statusWriter struct{ a *App }

func (w statusWriter) Write(p []byte) (int, error) {
	if msg := strings.Join(strings.Fields(string(p)), " "); msg != "" {
		w.a.model.SetStatus(msg, false, 4*time.Second)
	}
	return len(p), nil
}

// Run starts the interactive TUI event loop.
func (a *App) Run(ctx context.Context) (retErr error) {
	stdinFile, ok := a.cfg.Stdin.(*os.File)
	if !ok || !term.IsTerminal(int(stdinFile.Fd())) {
		return errors.New("portlens top requires an interactive terminal")
	}

	stdoutFile, ok := a.cfg.Stdout.(*os.File)
	if !ok || !term.IsTerminal(int(stdoutFile.Fd())) {
		return errors.New("portlens top output must be an interactive terminal")
	}

	// Make terminal raw
	oldState, err := term.MakeRaw(int(stdinFile.Fd()))
	if err != nil {
		return fmt.Errorf("entering terminal raw mode: %w", err)
	}

	// Fail-safe terminal cleanup
	defer func() {
		_ = term.Restore(int(stdinFile.Fd()), oldState)
		_, _ = a.cfg.Stdout.Write([]byte("\x1b[?25h\x1b[?1049l"))
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	// Enter alternate screen buffer and hide cursor
	_, _ = a.cfg.Stdout.Write([]byte("\x1b[?1049h\x1b[?25l"))

	// Initial size
	if w, h, szErr := term.GetSize(int(stdoutFile.Fd())); szErr == nil && w > 0 && h > 0 {
		a.model.Width = w
		a.model.Height = h
	}

	// Signal handling
	sigCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	winchChan := make(chan os.Signal, 1)
	signal.Notify(winchChan, syscall.SIGWINCH)
	defer signal.Stop(winchChan)

	// Background key event channel (buffered so reader never blocks)
	keyChan := make(chan Key, 32)
	errChan := make(chan error, 1)
	go func() {
		kr := NewKeyReader(stdinFile)
		for {
			k, kErr := kr.ReadKey()
			if kErr != nil {
				errChan <- kErr
				return
			}
			keyChan <- k
		}
	}()

	// Channels for async operations
	listResultChan := make(chan []model.PortEntry, 1)
	reportResultChan := make(chan *model.Report, 1)

	// Fetch initial port list immediately
	a.refreshList(sigCtx, listResultChan)

	ticker := time.NewTicker(a.cfg.Interval)
	defer ticker.Stop()

	// Initial render
	a.draw()

	for {
		select {
		case <-sigCtx.Done():
			return nil

		case kErr := <-errChan:
			if errors.Is(kErr, io.EOF) {
				return nil
			}
			return kErr

		case <-winchChan:
			if w, h, szErr := term.GetSize(int(stdoutFile.Fd())); szErr == nil && w > 0 && h > 0 {
				a.mu.Lock()
				a.model.Width = w
				a.model.Height = h
				a.mu.Unlock()
				a.draw()
			}

		case <-ticker.C:
			a.refreshList(sigCtx, listResultChan)

		case entries := <-listResultChan:
			a.mu.Lock()
			a.model.SetEntries(entries)
			a.ensureSelectedReportLocked(sigCtx, reportResultChan)
			a.mu.Unlock()
			a.draw()

		case rep := <-reportResultChan:
			a.mu.Lock()
			if rep != nil {
				a.model.ReportCache[rep.Port] = rep
				if sel := a.model.SelectedEntry(); sel != nil && sel.Port == rep.Port {
					a.model.SelectedReport = rep
				}
			}
			a.mu.Unlock()
			a.draw()

		case k := <-keyChan:
			shouldExit := a.handleKey(sigCtx, k, reportResultChan, listResultChan)
			if shouldExit {
				return nil
			}
			a.draw()
		}
	}
}

func (a *App) draw() {
	a.mu.Lock()
	frame := Render(a.model)
	a.mu.Unlock()
	_, _ = a.cfg.Stdout.Write([]byte(frame))
}

func (a *App) refreshList(ctx context.Context, out chan<- []model.PortEntry) {
	a.refresh.run(func() {
		entries, err := a.cfg.Service.List(ctx, a.cfg.OnlyTCP)
		if err == nil && entries != nil {
			select {
			case out <- entries:
			default:
			}
		}
	})
}

func (a *App) ensureSelectedReportLocked(ctx context.Context, out chan<- *model.Report) {
	sel := a.model.SelectedEntry()
	if sel == nil {
		a.model.SelectedReport = nil
		return
	}
	cached := a.model.ReportCache[sel.Port]
	if cached != nil {
		a.model.SelectedReport = cached
	}
	a.reportWant.Store(&reportTarget{port: sel.Port, proto: sel.Protocol})
	a.report.run(func() {
		t := a.reportWant.Load()
		rep, err := a.cfg.Service.Inspect(ctx, t.port, t.proto, inspector.DepthFull)
		if err == nil && rep != nil {
			select {
			case out <- rep:
			default:
			}
		}
	})
}

func (a *App) handleKey(ctx context.Context, k Key, reportChan chan<- *model.Report, listChan chan<- []model.PortEntry) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if k.Type == KeyCtrlC {
		return true
	}

	// 1. Help Modal Mode
	if a.model.ViewMode == ModeHelp {
		a.model.ViewMode = ModeNormal
		return false
	}

	// 2. Confirm Dialog Mode
	if a.model.ViewMode == ModeConfirm {
		return a.handleConfirmKey(ctx, k, listChan)
	}

	// 3. Filter Input Mode
	if a.model.ViewMode == ModeFilter {
		return a.handleFilterKey(ctx, k, reportChan)
	}

	// 4. Normal Navigation Mode
	return a.handleNormalKey(ctx, k, reportChan)
}

func (a *App) handleConfirmKey(ctx context.Context, k Key, listChan chan<- []model.PortEntry) bool {
	if k.Type == KeyEsc || (k.Type == KeyRune && (k.Rune == 'n' || k.Rune == 'N')) {
		a.model.ViewMode = ModeNormal
		a.model.ConfirmAction = ConfirmNone
		a.model.SetStatus("Action cancelled", false, 2*time.Second)
		return false
	}

	if k.Type == KeyEnter || (k.Type == KeyRune && (k.Rune == 'y' || k.Rune == 'Y')) {
		action := a.model.ConfirmAction
		target := a.model.ConfirmTarget
		rep := a.model.SelectedReport

		a.model.ViewMode = ModeNormal
		a.model.ConfirmAction = ConfirmNone

		if target == nil {
			a.model.SetStatus("No target selected", true, 3*time.Second)
			return false
		}

		go func() {
			var err error
			var successMsg string

			switch action {
			case ConfirmKill:
				if rep != nil {
					err = a.actions.Kill(ctx, rep, false)
				} else {
					err = a.actions.TerminateTree(ctx, target.PID, false)
				}
				successMsg = fmt.Sprintf("Gracefully terminated PID %d (%s)", target.PID, target.Process)

			case ConfirmForceKill:
				if rep != nil {
					err = a.actions.Kill(ctx, rep, true)
				} else {
					err = a.actions.TerminateTree(ctx, target.PID, true)
				}
				successMsg = fmt.Sprintf("Force killed PID %d (%s)", target.PID, target.Process)

			case ConfirmRestart:
				if rep == nil {
					err = errors.New("detailed process launch command not yet loaded")
				} else {
					err = a.actions.Restart(ctx, rep)
				}
				successMsg = fmt.Sprintf("Restarted %s", target.Process)
			}

			a.mu.Lock()
			if err != nil {
				a.model.SetStatus(err.Error(), true, 4*time.Second)
			} else {
				a.model.SetStatus(successMsg, false, 4*time.Second)
			}
			a.mu.Unlock()

			// Trigger list refresh to reflect state change
			a.refreshList(ctx, listChan)
		}()
		return false
	}

	return false
}

func (a *App) handleFilterKey(ctx context.Context, k Key, reportChan chan<- *model.Report) bool {
	switch k.Type {
	case KeyEnter:
		a.model.ViewMode = ModeNormal
	case KeyEsc:
		a.model.ViewMode = ModeNormal
		a.model.SetFilter("")
		a.ensureSelectedReportLocked(ctx, reportChan)
	case KeyBackspace, KeyDelete:
		if len(a.model.Filter) > 0 {
			a.model.SetFilter(a.model.Filter[:len(a.model.Filter)-1])
			a.ensureSelectedReportLocked(ctx, reportChan)
		}
	case KeyRune:
		a.model.SetFilter(a.model.Filter + string(k.Rune))
		a.ensureSelectedReportLocked(ctx, reportChan)
	}
	return false
}

func (a *App) handleNormalKey(ctx context.Context, k Key, reportChan chan<- *model.Report) bool {
	switch k.Type {
	case KeyEsc:
		if a.model.Filter != "" {
			a.model.SetFilter("")
			a.ensureSelectedReportLocked(ctx, reportChan)
		}
	case KeyRune:
		switch k.Rune {
		case 'q', 'Q':
			return true
		case '?':
			a.model.ViewMode = ModeHelp
		case '/':
			a.model.ViewMode = ModeFilter
		case 'j':
			a.model.MoveDown()
			a.ensureSelectedReportLocked(ctx, reportChan)
		case 'g':
			a.model.Home()
			a.ensureSelectedReportLocked(ctx, reportChan)
		case 'G':
			a.model.End()
			a.ensureSelectedReportLocked(ctx, reportChan)
		case '1':
			a.model.DetailTab = TabOverview
		case '2', 't', 'T':
			a.model.DetailTab = TabTree
		case '3', 'n', 'N':
			a.model.DetailTab = TabConnections
		case 'k', 'K': // Graceful kill
			a.promptKill(false)
		case 'f', 'F':
			a.promptKill(true)
		case 'r', 'R':
			a.promptRestart()
		case 'o', 'O':
			a.openBrowser(ctx)
		case 'c', 'C':
			a.copyPID(ctx)
		case 'u', 'U':
			a.copyURL(ctx)
		}

	case KeyUp:
		a.model.MoveUp()
		a.ensureSelectedReportLocked(ctx, reportChan)

	case KeyDown:
		a.model.MoveDown()
		a.ensureSelectedReportLocked(ctx, reportChan)

	case KeyPgUp:
		a.model.PageUp(a.model.Height - 6)
		a.ensureSelectedReportLocked(ctx, reportChan)

	case KeyPgDown:
		a.model.PageDown(a.model.Height - 6)
		a.ensureSelectedReportLocked(ctx, reportChan)

	case KeyHome:
		a.model.Home()
		a.ensureSelectedReportLocked(ctx, reportChan)

	case KeyEnd:
		a.model.End()
		a.ensureSelectedReportLocked(ctx, reportChan)
	}

	return false
}

func (a *App) promptKill(force bool) {
	sel := a.model.SelectedEntry()
	if sel == nil {
		a.model.SetStatus("No port selected to terminate", true, 3*time.Second)
		return
	}
	if sel.PID <= 0 {
		a.model.SetStatus(fmt.Sprintf("Cannot terminate port %d: PID unknown or insufficient permissions", sel.Port), true, 4*time.Second)
		return
	}

	a.model.ViewMode = ModeConfirm
	a.model.ConfirmTarget = sel
	if force {
		a.model.ConfirmAction = ConfirmForceKill
		a.model.ConfirmPrompt = fmt.Sprintf("Force kill (SIGKILL) %s (PID %d) on port %d?", sel.Process, sel.PID, sel.Port)
	} else {
		a.model.ConfirmAction = ConfirmKill
		a.model.ConfirmPrompt = fmt.Sprintf("Terminate %s (PID %d) on port %d with SIGTERM?", sel.Process, sel.PID, sel.Port)
	}
}

func (a *App) promptRestart() {
	sel := a.model.SelectedEntry()
	if sel == nil {
		a.model.SetStatus("No port selected to restart", true, 3*time.Second)
		return
	}
	if sel.PID <= 0 {
		a.model.SetStatus(fmt.Sprintf("Cannot restart port %d: PID unknown", sel.Port), true, 4*time.Second)
		return
	}

	a.model.ViewMode = ModeConfirm
	a.model.ConfirmTarget = sel
	a.model.ConfirmAction = ConfirmRestart
	a.model.ConfirmPrompt = fmt.Sprintf("Restart process %s (PID %d) on port %d?", sel.Process, sel.PID, sel.Port)
}

func (a *App) openBrowser(ctx context.Context) {
	sel := a.model.SelectedEntry()
	if sel == nil {
		a.model.SetStatus("No port selected to open", true, 3*time.Second)
		return
	}
	if report := a.model.SelectedReport; report != nil {
		if err := a.actions.Open(ctx, report); err != nil {
			a.model.SetStatus(fmt.Sprintf("Failed to open browser: %v", err), true, 4*time.Second)
		} else {
			a.model.SetStatus(fmt.Sprintf("Opened %s in browser", actions.LocalURL(report)), false, 3*time.Second)
		}
		return
	}
	// Report not loaded yet: fall back to the entry's port.
	url := fmt.Sprintf("http://localhost:%d", sel.Port)
	if err := platform.OpenURL(ctx, url); err != nil {
		a.model.SetStatus(fmt.Sprintf("Failed to open browser: %v", err), true, 4*time.Second)
	} else {
		a.model.SetStatus(fmt.Sprintf("Opened %s in browser", url), false, 3*time.Second)
	}
}

func (a *App) copyPID(ctx context.Context) {
	sel := a.model.SelectedEntry()
	if sel == nil || sel.PID <= 0 {
		a.model.SetStatus("No valid PID to copy", true, 3*time.Second)
		return
	}
	pidStr := strconv.Itoa(int(sel.PID))
	if err := a.actions.Copy(ctx, pidStr); err != nil {
		a.model.SetStatus(fmt.Sprintf("Clipboard error: %v", err), true, 4*time.Second)
	} else {
		a.model.SetStatus(fmt.Sprintf("Copied PID %s to clipboard", pidStr), false, 3*time.Second)
	}
}

func (a *App) copyURL(ctx context.Context) {
	sel := a.model.SelectedEntry()
	if sel == nil {
		a.model.SetStatus("No port selected", true, 3*time.Second)
		return
	}
	url := fmt.Sprintf("http://localhost:%d", sel.Port)
	if a.model.SelectedReport != nil {
		url = actions.LocalURL(a.model.SelectedReport)
	}
	if err := a.actions.Copy(ctx, url); err != nil {
		a.model.SetStatus(fmt.Sprintf("Clipboard error: %v", err), true, 4*time.Second)
	} else {
		a.model.SetStatus(fmt.Sprintf("Copied %s to clipboard", url), false, 3*time.Second)
	}
}

// RunTop launches the TUI dashboard with the provided CLI options.
func RunTop(ctx context.Context, interval int, tcpOnly bool, stdout io.Writer, stdin io.Reader) int {
	plat := platform.New()
	insp := inspector.New(plat)
	svc := service.New(
		service.WithInspector(insp),
	)

	dur := time.Duration(interval) * time.Second
	if interval <= 0 {
		dur = 2 * time.Second
	}

	app := NewApp(AppConfig{
		Service:  svc,
		Platform: plat,
		Interval: dur,
		OnlyTCP:  tcpOnly,
		Stdout:   stdout,
		Stdin:    stdin,
	})

	if err := app.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "portlens top: %v\n", err)
		return exitcode.GeneralError
	}
	return exitcode.Success
}
