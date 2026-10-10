package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
)

// Restart relaunches the owning process (or restarts the owning container). For
// containers it issues a runtime restart; otherwise it re-runs the raw argv of
// the shell-launched process directly (never through a shell, so a crafted
// argv cannot inject shell syntax) in the process's working directory. It only
// runs when the target can be confidently identified.
func (m *Manager) Restart(ctx context.Context, report *model.Report) error {
	if c := m.lookupContainer(ctx, report); c != nil {
		return m.restartContainer(ctx, c)
	}
	proc := inspector.LaunchProcess(report)
	if proc == nil {
		return ErrRestartUnavailable
	}
	// The ancestor chain carries only identity (name/pid/ppid); fetch the full
	// argv when the recorded Cmdline is empty so restart still works with the
	// native process-table provider.
	argv := proc.Cmdline
	if len(argv) == 0 && m.Platform != nil && m.Platform.Processes != nil {
		if full, err := m.Platform.Processes.Info(ctx, proc.PID); err == nil && len(full.Cmdline) > 0 {
			proc = full
			argv = full.Cmdline
		}
	}
	if len(argv) == 0 {
		return ErrRestartUnavailable
	}
	cwd := ""
	if report.Process != nil {
		cwd = report.Process.CWD
	}

	fmt.Fprintf(m.Out, "Detected command:\n  %s\n\nRestart command:\n  %s\n", proc.Command, proc.Command)

	if m.Confirm != nil {
		ok, err := m.Confirm("Restart the process? [y/N] ")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted by user")
		}
	}

	// 1. Terminate the existing process first so that the new process does not
	// fail with an address-already-in-use error.
	if report.Process != nil && report.Process.PID > 0 {
		fmt.Fprintf(m.Out, "Stopping existing process %d (%s)...\n", report.Process.PID, report.Process.Name)
		if err := m.TerminateTree(ctx, report.Process.PID, false); err != nil {
			return fmt.Errorf("failed to stop existing process %d: %w", report.Process.PID, err)
		}
		// Brief wait for the port socket to be fully released by the OS.
		if m.Platform != nil && m.Platform.Ports != nil && report.Port > 0 {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				listeners, err := m.Platform.Ports.ResolvePort(ctx, uint16(report.Port), report.Protocol)
				if err != nil || len(listeners) == 0 {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	starter := m.Starter
	if starter == nil {
		starter = defaultProcessStarter
	}
	newPID, err := starter(ctx, argv, cwd)
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}
	fmt.Fprintf(m.Out, "Restarted with pid %d (running detached)\n", newPID)
	return nil
}

// defaultProcessStarter spawns a process detached from the current terminal
// session with stdio connected to os.DevNull so background logs do not bleed into the shell.
func defaultProcessStarter(ctx context.Context, argv []string, cwd string) (int, error) {
	run := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if cwd != "" {
		run.Dir = cwd
	}
	// Detach the restarted process from our own lifetime so it keeps running
	// after PortLens exits.
	run.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		defer devNull.Close()
		run.Stdin = devNull
		run.Stdout = devNull
		run.Stderr = devNull
	}

	if err := run.Start(); err != nil {
		return 0, err
	}
	return run.Process.Pid, nil
}

// restartContainer restarts a container via the container runtime.
func (m *Manager) restartContainer(ctx context.Context, c *model.Container) error {
	name := containerActionName(c)
	fmt.Fprintf(m.Out, "Restarting container %s\n", name)
	if m.Confirm != nil {
		ok, err := m.Confirm(fmt.Sprintf("Restart container %s? [y/N] ", name))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted by user")
		}
	}
	if err := m.Platform.Containers.Restart(ctx, c.ID, 10*time.Second); err != nil {
		return fmt.Errorf("restart container %s: %w", name, err)
	}
	fmt.Fprintf(m.Out, "Container %s restarted\n", name)
	return nil
}
