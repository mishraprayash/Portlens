package inspector

import (
	"strings"

	"github.com/portlens/portlens/internal/model"
)

var shellNames = map[string]bool{
	"zsh": true, "bash": true, "sh": true, "fish": true,
	"dash": true, "ksh": true, "csh": true, "tcsh": true,
	"nu": true, "elvish": true, "pwsh": true,
}

// LaunchProcess returns the process that was launched directly by an
// interactive shell, whose raw argv would be re-run to restart the tree. It
// returns nil when no shell-launched ancestor is found (daemons, services
// launched by init/launchd/systemd). It scans ancestors from the target toward
// the root so that the *nearest* shell is used, which matters when shells are
// nested (e.g. Terminal → zsh → tool → zsh → target).
func LaunchProcess(report *model.Report) *model.ProcessInfo {
	ancestors := report.Ancestors // oldest first
	for i := len(ancestors) - 1; i >= 0; i-- {
		a := ancestors[i]
		if a == nil {
			continue
		}
		if isShell(a.Name) {
			if i+1 < len(ancestors) && ancestors[i+1] != nil {
				return ancestors[i+1]
			}
			if report.Process != nil && report.Process.PID != a.PID {
				return report.Process
			}
			// The shell itself owns the port; its own argv is the entry point.
			return a
		}
	}
	return nil
}

// LaunchCommand attempts to determine the command the user actually typed to
// start this process tree. It looks for the first ancestor launched directly by
// an interactive shell. If no shell ancestor is found (daemons, services
// launched by init/launchd/systemd), it returns "" so that automatic restart is
// not attempted.
func LaunchCommand(report *model.Report) string {
	if p := LaunchProcess(report); p != nil {
		return p.Command
	}
	return ""
}

func isShell(name string) bool {
	return shellNames[strings.ToLower(name)]
}
