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

// LaunchCommand attempts to determine the command the user actually typed to
// start this process tree. It looks for the first ancestor launched directly by
// an interactive shell. If no shell ancestor is found (daemons, services
// launched by init/launchd/systemd), it returns "" so that automatic restart is
// not attempted.
func LaunchCommand(report *model.Report) string {
	ancestors := report.Ancestors // oldest first
	for i := 0; i < len(ancestors); i++ {
		a := ancestors[i]
		if a == nil {
			continue
		}
		if isShell(a.Name) {
			if i+1 < len(ancestors) && ancestors[i+1] != nil {
				if cmd := ancestors[i+1].Command; cmd != "" {
					return cmd
				}
			}
			// If the shell directly owns the port, its own command is the
			// entry point.
			return a.Command
		}
	}
	return ""
}

func isShell(name string) bool {
	return shellNames[strings.ToLower(name)]
}
