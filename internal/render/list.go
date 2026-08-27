package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/model"
)

// ListOptions controls the port listing table.
type ListOptions struct {
	SortBy  string // "port" (default) | "process" | "project" | "runtime"
	Filter  string // case-insensitive substring filter across all columns
	OnlyTCP bool
}

// List renders a table of listening ports.
func (r *Renderer) List(entries []model.PortEntry, opts ListOptions) {
	if opts.OnlyTCP {
		entries = filterEntries(entries, func(e model.PortEntry) bool {
			return e.Protocol.Normalize() == model.ProtocolTCP
		})
	}
	if opts.Filter != "" {
		f := strings.ToLower(opts.Filter)
		entries = filterEntries(entries, func(e model.PortEntry) bool {
			hay := strings.ToLower(fmt.Sprintf("%d %s %s %s %s %s",
				e.Port, e.Process, e.Project, e.Runtime, e.Address, e.Status))
			return strings.Contains(hay, f)
		})
	}

	sortEntries(entries, opts.SortBy)

	if len(entries) == 0 {
		r.writeln(r.dim("No listening ports found."))
		return
	}

	headers := []string{"PORT", "PROCESS", "PROJECT", "RUNTIME", "ADDRESS", "STATUS"}
	cols := [][]string{}
	for _, e := range entries {
		rt := e.Runtime
		if label := detect.ShortRuntimeName(rt); label != "" {
			rt = label
		}
		if rt == "" {
			rt = "-"
		}
		proc := e.Process
		if proc == "" {
			proc = "-"
		}
		proj := e.Project
		if proj == "" {
			proj = "-"
		}
		cols = append(cols, []string{
			fmt.Sprintf("%d", e.Port),
			proc,
			proj,
			rt,
			formatAddr(e.Address, uint16(e.Port)),
			e.Status,
		})
	}
	r.writeln(r.table(headers, cols))
}

func filterEntries(in []model.PortEntry, keep func(model.PortEntry) bool) []model.PortEntry {
	out := in[:0:0]
	for _, e := range in {
		if keep(e) {
			out = append(out, e)
		}
	}
	return out
}

func sortEntries(entries []model.PortEntry, by string) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		switch by {
		case "process":
			if a.Process != b.Process {
				return a.Process < b.Process
			}
			return a.Port < b.Port
		case "project":
			if a.Project != b.Project {
				return a.Project < b.Project
			}
			return a.Port < b.Port
		case "runtime":
			if a.Runtime != b.Runtime {
				return a.Runtime < b.Runtime
			}
			return a.Port < b.Port
		default:
			return a.Port < b.Port
		}
	})
}

// table renders a simple aligned ASCII table with a dim header row.
func (r *Renderer) table(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	var sb strings.Builder
	sb.WriteString(r.renderTableRow(headers, widths, true))
	sb.WriteString("\n")
	for _, row := range rows {
		sb.WriteString(r.renderTableRow(row, widths, false))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (r *Renderer) renderTableRow(cells []string, widths []int, header bool) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		parts[i] = padRight(c, widths[i])
	}
	line := strings.Join(parts, "  ")
	if header {
		return r.bold(line)
	}
	return line
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
