package render

import (
	"fmt"
	"strings"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/model"
)

// Summary renders a compact at-a-glance overview of a single port: status,
// address, owning process, project, and exposure. It is the default view for
// `portlens <port>`; use --verbose to get the full Report.
func (r *Renderer) Summary(report *model.Report) {
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
	if report.Process != nil {
		p := report.Process
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("pid %d", p.PID)
		}
		rows = append(rows, [2]string{"Process", fmt.Sprintf("%s (pid %d)", name, p.PID)})
		if p.Command != "" {
			rows = append(rows, [2]string{"Command", p.Command})
		}
	}
	if c := report.Container; c != nil {
		rows = append(rows, [2]string{"Container", containerLabel(c)})
	}
	if report.Project != nil && (report.Project.Detected || report.Project.Name != "" || report.Project.Runtime != "") {
		label := report.Project.Name
		rt := report.Project.Runtime
		if label == "" {
			label = rt
			rt = ""
		}
		if rt != "" {
			if short := detect.ShortRuntimeName(rt); short != "" {
				rt = short
			}
			label += "  (" + rt + ")"
		}
		rows = append(rows, [2]string{"Project", label})
	}
	if report.Exposure != nil && report.Exposure.Worst != "" {
		rows = append(rows, [2]string{"Exposure", string(report.Exposure.Worst)})
	}
	r.writeln(r.kv(rows))
	r.writeln("")
}
