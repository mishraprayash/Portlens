package render

import (
	"fmt"
	"strings"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/model"
)

// Report renders the full human-facing inspection for a single port.
func (r *Renderer) Report(report *model.Report) {
	if report == nil {
		return
	}
	if report.Status != "listening" {
		r.renderNotListening(report)
		return
	}

	r.writeln(r.bold(fmt.Sprintf("PORT %d", report.Port)))
	r.writeln(r.hr())

	rows := [][2]string{
		{"Status", strings.ToUpper(report.Status)},
		{"Protocol", strings.ToUpper(string(report.Protocol.Normalize()))},
		{"Address", formatAddr(report.Address, uint16(report.Port))},
	}
	if report.Service != "" {
		rows = append(rows, [2]string{"Service", report.Service})
	}
	r.writeln(r.kv(rows))

	if report.Process != nil {
		r.renderProcess(report)
	}
	if report.Container != nil {
		r.renderContainer(report.Container)
	}
	if report.Project != nil && (report.Project.Detected || report.Project.Runtime != "" || report.Project.GitRepo != "") {
		r.renderProject(report.Project)
	}
	if report.Exposure != nil {
		r.renderExposure(report)
	}
	if report.HTTPProbe != nil {
		r.renderHTTPProbe(report.HTTPProbe)
	}
	if report.Process != nil {
		r.renderTreeSection(report)
	}
	if report.Network != nil {
		r.renderNetwork(report)
	}
	r.renderInterpretation(report)
	r.renderActions()
}

func (r *Renderer) renderNotListening(report *model.Report) {
	r.writeln(r.bold(fmt.Sprintf("PORT %d", report.Port)))
	r.writeln(r.hr())
	r.writeln(r.yellow("No process is currently listening on this port."))
	r.writeln(r.dim("Try:"))
	r.writeln(r.dim("  portlens            list active listening ports"))
	r.writeln(r.dim("  portlens next       find the next available port"))
}

func (r *Renderer) renderProcess(report *model.Report) {
	p := report.Process
	r.section("PROCESS", func() {
		rows := [][2]string{
			{"PID", fmt.Sprintf("%d", p.PID)},
			{"Name", p.Name},
			{"Command", p.Command},
		}
		if report.Origin != "" {
			rows = append(rows, [2]string{"Origin", string(report.Origin)})
		}
		if !p.StartTime.IsZero() {
			rows = append(rows, [2]string{"Started", p.StartTime.Format("15:04:05")},
				[2]string{"Runtime", p.Runtime(reportNow())})
		}
		if p.User != "" {
			rows = append(rows, [2]string{"User", p.User})
		}
		if p.MemoryBytes > 0 {
			rows = append(rows, [2]string{"Memory", model.FormatBytes(p.MemoryBytes)})
		}
		if p.CWD != "" {
			rows = append(rows, [2]string{"Directory", shortenHome(p.CWD)})
		}
		if parent := parentLabel(report); parent != "" {
			rows = append(rows, [2]string{"Parent", parent})
		}
		r.writeln(r.kv(rows))
	})
}

func (r *Renderer) renderContainer(c *model.Container) {
	r.section("CONTAINER", func() {
		rows := [][2]string{
			{"Name", c.Name},
			{"ID", shortID(c.ID)},
			{"Image", c.Image},
		}
		if c.Status != "" {
			rows = append(rows, [2]string{"Status", c.Status})
		}
		if c.ComposeProject != "" {
			rows = append(rows, [2]string{"Compose Project", c.ComposeProject})
		}
		if c.ComposeService != "" {
			rows = append(rows, [2]string{"Compose Service", c.ComposeService})
		}
		r.writeln(r.kv(rows))
	})
}

// shortID trims a container ID to its leading 12 characters for readability.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (r *Renderer) renderProject(p *model.ProjectInfo) {
	r.section("PROJECT", func() {
		rows := [][2]string{}
		if p.Directory != "" {
			rows = append(rows, [2]string{"Directory", shortenHome(p.Directory)})
		}
		if p.GitRepo != "" {
			repo := p.GitRepo
			if p.GitBranch != "" {
				repo += fmt.Sprintf("  (%s)", p.GitBranch)
			}
			rows = append(rows, [2]string{"Git Repo", repo})
		}
		if p.Runtime != "" {
			label := detect.ShortRuntimeName(p.Runtime)
			if label == "" {
				label = p.Runtime
			}
			rows = append(rows, [2]string{"Runtime", label})
		}
		if p.Framework != "" {
			rows = append(rows, [2]string{"Framework", detect.FrameworkDisplay(p.Framework)})
		}
		if p.PackageManager != "" {
			rows = append(rows, [2]string{"Package Mgr", p.PackageManager})
		}
		r.writeln(r.kv(rows))
	})
}

func (r *Renderer) renderExposure(report *model.Report) {
	e := report.Exposure
	r.section("EXPOSURE", func() {
		for _, f := range e.Findings {
			icon, style := riskStyle(r, f.Level)
			r.writeln(icon + " " + style(string(f.Level)) + "  " + r.dim(f.Reason))
		}
	})
}

func riskStyle(r *Renderer, level model.RiskLevel) (string, func(string) string) {
	switch level {
	case model.RiskLow:
		return r.green("✓"), r.green
	case model.RiskDangerous:
		return r.red("✗"), r.red
	default:
		return r.yellow("⚠"), r.yellow
	}
}

func (r *Renderer) renderTreeSection(report *model.Report) {
	tree := r.renderProcessTree(report)
	if tree == "" {
		return
	}
	r.section("PROCESS TREE", func() {
		r.writeln(tree)
	})
}

func (r *Renderer) renderNetwork(report *model.Report) {
	n := report.Network
	r.section("NETWORK", func() {
		rows := [][2]string{
			{"Connections", fmt.Sprintf("%d", n.Summary.Total)},
		}
		for _, state := range []string{"ESTABLISHED", "TIME_WAIT", "CLOSE_WAIT", "LISTEN"} {
			if c := n.Summary.ByState[state]; c > 0 {
				rows = append(rows, [2]string{state, fmt.Sprintf("%d", c)})
			}
		}
		r.writeln(r.kv(rows))
	})
}

func (r *Renderer) renderInterpretation(report *model.Report) {
	r.section("INTERPRETATION", func() {
		if report.Interpretation != "" {
			r.writeln(report.Interpretation)
			r.writeln("")
		}
		r.writeln(r.bold("Facts"))
		for _, f := range report.Facts {
			r.writeln("  " + f)
		}
		if len(report.Inferences) > 0 {
			r.writeln("")
			r.writeln(r.bold("Inferred") + r.dim("  (best-effort, not guaranteed)"))
			for _, inf := range report.Inferences {
				r.writeln("  " + inf)
			}
		}
	})
}

func (r *Renderer) renderActions() {
	r.section("ACTIONS", func() {
		r.writeln(r.dim("[k]") + " Kill gracefully")
		r.writeln(r.dim("[f]") + " Force kill")
		r.writeln(r.dim("[r]") + " Restart")
		r.writeln(r.dim("[o]") + " Open localhost")
		r.writeln(r.dim("[c]") + " Copy PID")
		r.writeln(r.dim("[u]") + " Copy local URL")
		r.writeln(r.dim("[t]") + " Process tree")
		r.writeln(r.dim("[n]") + " Connections")
		r.writeln(r.dim("[q]") + " Quit")
	})
}

func formatAddr(addr string, port uint16) string {
	if strings.Contains(addr, ":") {
		return fmt.Sprintf("[%s]:%d", addr, port)
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func shortenHome(path string) string {
	home := userHomeDir()
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func parentLabel(report *model.Report) string {
	if len(report.Ancestors) < 2 {
		return ""
	}
	parent := report.Ancestors[len(report.Ancestors)-2]
	if parent == nil || parent.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s (pid %d)", parent.Name, parent.PID)
}

func (r *Renderer) renderHTTPProbe(p *model.HTTPProbe) {
	r.section("HTTP PROBE", func() {
		rows := [][2]string{
			{"Status", fmt.Sprintf("%s (%s)", p.Status, model.FormatDuration(p.Latency))},
		}
		if p.Title != "" {
			rows = append(rows, [2]string{"HTML Title", p.Title})
		}
		if p.Server != "" {
			rows = append(rows, [2]string{"Server", p.Server})
		}
		r.writeln(r.kv(rows))
	})
}
