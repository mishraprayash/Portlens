package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/portlens/portlens/internal/model"
)

// Restart launches the detected launch command again in the process's working
// directory. It only runs when the launch command can be confidently inferred.
func (m *Manager) Restart(ctx context.Context, report *model.Report) error {
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
