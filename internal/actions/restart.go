package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/portlens/portlens/internal/model"
)

// Restart relaunches the owning process (or restarts the owning container). For
// containers it issues a runtime restart; otherwise it runs the detected launch
// command again in the process's working directory. It only runs when the
// target can be confidently identified.
func (m *Manager) Restart(ctx context.Context, report *model.Report) error {
	if c := m.lookupContainer(ctx, report); c != nil {
		return m.restartContainer(ctx, c)
	}
	cmd, err := RestartCommand(report)
	if err != nil {
		return err
	}
	cwd := ""
	if report.Process != nil {
		cwd = report.Process.CWD
	}

	fmt.Fprintf(m.Out, "Detected command:\n  %s\n\nRestart command:\n  %s\n", cmd, cmd)

	if m.Confirm != nil {
		ok, err := m.Confirm("Restart the process? [y/N] ")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted by user")
		}
	}

	run := exec.CommandContext(ctx, "sh", "-c", cmd)
	if cwd != "" {
		run.Dir = cwd
	}
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	// Detach the restarted process from our own lifetime so it keeps running
	// after PortLens exits.
	run.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := run.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}
	fmt.Fprintf(m.Out, "Restarted with pid %d\n", run.Process.Pid)
	// Do not wait: the restarted process is expected to be long-running.
	return nil
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
