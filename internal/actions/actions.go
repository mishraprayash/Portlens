// Package actions implements the state-changing operations PortLens can perform:
// graceful/forced termination, restart, opening a URL, and copying to the
// clipboard. All destructive operations confirm before acting and never
// escalate privileges.
package actions

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// ConfirmFunc prompts the user and reports whether to proceed.
type ConfirmFunc func(prompt string) (bool, error)

// Manager coordinates actions over a platform handle.
type Manager struct {
	Platform *platform.Platform
	Confirm  ConfirmFunc
	Out      io.Writer
	Wait     time.Duration
}

// NewManager builds a Manager with sensible defaults.
func NewManager(p *platform.Platform, out io.Writer, confirm ConfirmFunc) *Manager {
	return &Manager{Platform: p, Out: out, Confirm: confirm, Wait: 3 * time.Second}
}

// ErrStillRunning is returned when a graceful termination did not cause the
// process to exit within the wait window.
type ErrStillRunning struct {
	PID int32
}

func (e *ErrStillRunning) Error() string {
	return fmt.Sprintf("process %d is still running after graceful termination", e.PID)
}

// ErrRestartUnavailable is returned when PortLens cannot confidently determine
// how a process was launched.
var ErrRestartUnavailable = fmt.Errorf("automatic restart unavailable: cannot determine how the process was launched")

// TreePIDs returns the target PID and all descendant PIDs, deepest first.
func TreePIDs(tree *model.ProcessTree) []int32 {
	if tree == nil {
		return nil
	}
	var out []int32
	var walk func(*model.ProcessTree)
	walk = func(n *model.ProcessTree) {
		for _, c := range n.Children {
			walk(c)
		}
		out = append(out, n.Process.PID)
	}
	walk(tree)
	return out
}

// Kill terminates the process owning the report's port. When force is false it
// sends SIGTERM to the whole tree, waits for the owner to exit, and reports the
// outcome. When force is true it sends SIGKILL immediately without prompting.
func (m *Manager) Kill(ctx context.Context, report *model.Report, force bool) error {
	if report == nil || report.Process == nil {
		return fmt.Errorf("no owning process to terminate")
	}
	pids := []int32{report.Process.PID}
	if tree, err := m.Platform.Tree.Descendants(ctx, report.Process.PID); err == nil {
		pids = TreePIDs(tree)
	}
	if len(pids) == 1 {
		fmt.Fprintf(m.Out, "Terminating process %d (%s)\n", report.Process.PID, report.Process.Name)
	} else {
		fmt.Fprintf(m.Out, "Terminating process %d (%s) and %d descendant(s)\n",
			report.Process.PID, report.Process.Name, len(pids)-1)
	}

	if !force && m.Confirm != nil {
		ok, err := m.Confirm("Send SIGTERM to the process tree? [y/N] ")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted by user")
		}
	}

	sig := platform.SignalTerm
	label := "SIGTERM"
	if force {
		sig = platform.SignalKill
		label = "SIGKILL"
	}
	fmt.Fprintf(m.Out, "Sending %s...\n", label)

	// Signal deepest-first so children are cleaned up before their parent.
	var firstErr error
	for _, pid := range pids {
		if err := m.Platform.Controller.Signal(ctx, pid, sig); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if force {
		// SIGKILL is immediate; give the OS a moment then confirm.
		time.Sleep(300 * time.Millisecond)
		if m.Platform.Controller.IsAlive(ctx, report.Process.PID) {
			return &ErrStillRunning{PID: report.Process.PID}
		}
		fmt.Fprintf(m.Out, "Process %d terminated\n", report.Process.PID)
		return nil
	}

	if firstErr != nil {
		return firstErr
	}

	deadline := time.Now().Add(m.Wait)
	for time.Now().Before(deadline) {
		if !m.Platform.Controller.IsAlive(ctx, report.Process.PID) {
			fmt.Fprintf(m.Out, "Process %d exited gracefully\n", report.Process.PID)
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return &ErrStillRunning{PID: report.Process.PID}
}

// RestartCommand determines the command that can be used to restart the process
// tree, if one can be confidently inferred.
func RestartCommand(report *model.Report) (string, error) {
	if report == nil || report.Process == nil {
		return "", ErrRestartUnavailable
	}
	cmd := inspector.LaunchCommand(report)
	if cmd == "" {
		return "", ErrRestartUnavailable
	}
	return cmd, nil
}

// Copy sends text to the system clipboard.
func (m *Manager) Copy(ctx context.Context, text string) error {
	return m.Platform.Clipboard.Copy(ctx, text)
}
