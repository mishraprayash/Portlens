package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"

	"golang.org/x/term"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/render"
)

// runInteractive renders the report and enters a single-key action loop. It is
// only entered when both stdout and stdin are terminals. The landing view uses
// the fast report; tree and connections re-inspect with full depth on demand
// so the common case never pays for data it does not display.
func runInteractive(
	ctx context.Context,
	insp *inspector.Inspector,
	mgr *actions.Manager,
	r *render.Renderer,
	report *model.Report,
	stdin io.Reader,
	stdout io.Writer,
	opts *options,
) int {
	// The interactive loop still offers full detail via keys; the default
	// landing view stays compact unless --verbose is given.
	renderReport(r, report, opts)

	f, ok := stdin.(*os.File)
	if !ok {
		return exitcode.Success
	}

	for {
		fmt.Fprintln(stdout)
		fmt.Fprint(stdout, "  [k]ill  [f]orce  [r]estart  [o]pen  [c]opy pid  [u]rl  [t]ree  [n]et  [q]uit: ")

		key, err := readRawKey(f)
		if err != nil {
			return exitcode.Success
		}
		fmt.Fprintln(stdout)

		switch key {
		case 'q', '\x03': // 'q' or Ctrl-C
			return exitcode.Success
		case '?':
			printKeyHelp(stdout)
		case 'c':
			copyText(ctx, mgr, stdout, fmt.Sprintf("%d", report.Process.PID), "PID")
		case 'u':
			copyText(ctx, mgr, stdout, actions.LocalURL(report), "URL")
		case 't':
			r.Tree(deepInteractiveReport(ctx, insp, report))
		case 'n':
			r.Connections(deepInteractiveReport(ctx, insp, report))
		case 'k':
			if report.Process != nil && !insp.Platform.Processes.Exists(ctx, report.Process.PID) {
				fmt.Fprintf(stdout, "Process %d is no longer running\n", report.Process.PID)
				return exitcode.Success
			}
			if err := mgr.Kill(ctx, report, false); err != nil {
				fmt.Fprintf(stdout, "kill failed: %v\n", err)
			}
			return exitcode.Success
		case 'f':
			if report.Process != nil && !insp.Platform.Processes.Exists(ctx, report.Process.PID) {
				fmt.Fprintf(stdout, "Process %d is no longer running\n", report.Process.PID)
				return exitcode.Success
			}
			if err := mgr.Kill(ctx, report, true); err != nil {
				fmt.Fprintf(stdout, "force kill failed: %v\n", err)
			}
			return exitcode.Success
		case 'r':
			if err := mgr.Restart(ctx, report); err != nil {
				fmt.Fprintf(stdout, "restart failed: %v\n", err)
			}
			return exitcode.Success
		case 'o':
			if err := mgr.Open(ctx, report); err != nil {
				fmt.Fprintf(stdout, "open failed: %v\n", err)
			}
		}
	}
}

// deepInteractiveReport re-inspects the current port at full depth so the
// interactive tree/connections views have fresh deep data. It falls back to the
// original report if the inspection fails (e.g. the process exited meanwhile).
func deepInteractiveReport(ctx context.Context, insp *inspector.Inspector, report *model.Report) *model.Report {
	deep, err := insp.InspectDepth(ctx, report.Port, report.Protocol, inspector.DepthFull)
	if err != nil {
		return report
	}
	return deep
}

func copyText(ctx context.Context, mgr *actions.Manager, stdout io.Writer, text, label string) {
	if text == "" {
		fmt.Fprintf(stdout, "nothing to copy\n")
		return
	}
	if err := mgr.Copy(ctx, text); err != nil {
		fmt.Fprintf(stdout, "could not copy %s: %v\n", label, err)
		return
	}
	fmt.Fprintf(stdout, "copied %s to clipboard: %s\n", label, text)
}

func printKeyHelp(w io.Writer) {
	fmt.Fprint(w, `  k  kill gracefully (SIGTERM)
  f  force kill (SIGKILL)
  r  restart (if launch command is known)
  o  open in browser
  c  copy PID
  u  copy local URL
  t  show process tree
  n  show connections
  q  quit
`)
}

func readRawKey(f *os.File) (byte, error) {
	fd := int(f.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return 0, err
	}
	defer term.Restore(fd, old)

	buf := make([]byte, 1)
	if _, err := f.Read(buf); err != nil {
		return 0, err
	}
	// If the user triggered an escape sequence (e.g. arrow keys \x1b[A), drain
	// trailing bytes so they don't leak into the terminal session after raw mode ends.
	if buf[0] == '\x1b' {
		drainEscape(f)
	}
	return buf[0], nil
}

func drainEscape(f *os.File) {
	fd := int(f.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		return
	}
	defer func() { _ = syscall.SetNonblock(fd, false) }()
	buf := make([]byte, 32)
	_, _ = f.Read(buf)
}
