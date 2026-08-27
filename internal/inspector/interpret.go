package inspector

import (
	"fmt"
	"strings"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/model"
)

// interpret fills the Facts, Inferences, and Interpretation fields of a report.
// Facts are concrete observations; Inferences are guesses that must never be
// presented as certain.
func (i *Inspector) interpret(report *model.Report) {
	p := report.Process
	if p == nil {
		return
	}

	report.Facts = append(report.Facts,
		fmt.Sprintf("Process %d (%s) is listening on %s:%d over %s",
			p.PID, p.Name, displayAddr(report.Address), report.Port, report.Protocol.Normalize()),
	)
	if p.Command != "" {
		report.Facts = append(report.Facts, fmt.Sprintf("Full command: %s", p.Command))
	}
	if !p.StartTime.IsZero() {
		report.Facts = append(report.Facts,
			fmt.Sprintf("Started at %s (%s ago)", p.StartTime.Format("15:04:05"), p.Runtime(i.Now())))
	}
	if p.User != "" {
		report.Facts = append(report.Facts, fmt.Sprintf("Running as user %q", p.User))
	}
	if p.CWD != "" {
		report.Facts = append(report.Facts, fmt.Sprintf("Working directory: %s", p.CWD))
	}
	if parent := parentName(report); parent != "" {
		report.Facts = append(report.Facts, fmt.Sprintf("Parent process: %s", parent))
	}

	runtime := ""
	framework := ""
	if report.Project != nil {
		runtime = report.Project.Runtime
		framework = report.Project.Framework
	}
	if runtime == "" {
		runtime = detect.DetectRuntime(p)
	}

	parts := []string{}
	if runtime != "" {
		parts = append(parts, detect.ShortRuntimeName(runtime))
	}
	if framework != "" {
		parts = append(parts, detect.FrameworkDisplay(framework))
	}
	if len(parts) > 0 {
		report.Inferences = append(report.Inferences,
			fmt.Sprintf("Appears to be a %s process", strings.Join(parts, " ")))
	}
	if report.Project != nil && report.Project.GitRepo != "" {
		msg := fmt.Sprintf("Belongs to git repository %q", report.Project.GitRepo)
		if report.Project.GitBranch != "" {
			msg += fmt.Sprintf(" on branch %q", report.Project.GitBranch)
		}
		report.Inferences = append(report.Inferences, msg)
	}
	if cmd := LaunchCommand(report); cmd != "" {
		report.Inferences = append(report.Inferences, fmt.Sprintf("Launched via %q", cmd))
	}

	report.Interpretation = buildInterpretation(runtime, framework)
}

func parentName(report *model.Report) string {
	if len(report.Ancestors) < 2 {
		return ""
	}
	parent := report.Ancestors[len(report.Ancestors)-2]
	if parent == nil {
		return ""
	}
	return fmt.Sprintf("%s (pid %d)", parent.Name, parent.PID)
}

func buildInterpretation(runtime, framework string) string {
	switch {
	case framework != "" && runtime != "":
		return fmt.Sprintf("%s process (%s)", detect.FrameworkDisplay(framework), detect.ShortRuntimeName(runtime))
	case runtime != "":
		return fmt.Sprintf("%s process", detect.ShortRuntimeName(runtime))
	case framework != "":
		return fmt.Sprintf("%s process", detect.FrameworkDisplay(framework))
	default:
		return "Unknown process"
	}
}
