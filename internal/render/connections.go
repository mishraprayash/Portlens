package render

import (
	"fmt"
	"sort"

	"github.com/portlens/portlens/internal/model"
)

// Connections renders a summary of a process's active connections, grouped by
// state to avoid dumping huge raw tables.
func (r *Renderer) Connections(report *model.Report) {
	if report == nil || report.Network == nil {
		return
	}
	n := report.Network

	pid := int32(0)
	if report.Process != nil {
		pid = report.Process.PID
	}
	r.writeln(r.bold(fmt.Sprintf("CONNECTIONS — PID %d", pid)))
	r.writeln(r.hr())

	if n.Summary.Total == 0 {
		r.writeln(r.dim("No active connections."))
		return
	}

	r.writeln(r.kv([][2]string{
		{"Total", fmt.Sprintf("%d", n.Summary.Total)},
	}))

	states := make([]string, 0, len(n.Summary.ByState))
	for s := range n.Summary.ByState {
		states = append(states, s)
	}
	sort.Strings(states)
	for _, s := range states {
		r.writeln(r.kv([][2]string{{s, fmt.Sprintf("%d", n.Summary.ByState[s])}}))
	}
	r.writeln("")

	// Show a representative sample, capped for readability.
	const maxRows = 25
	headers := []string{"LOCAL", "REMOTE", "STATE"}
	var rows [][]string
	for i, c := range n.Connections {
		if i >= maxRows {
			break
		}
		remote := formatAddr(c.RemoteAddr, c.RemotePort)
		if c.RemoteAddr == "" || c.RemotePort == 0 {
			remote = "*"
		}
		rows = append(rows, []string{
			formatAddr(c.LocalAddr, c.LocalPort),
			remote,
			c.State,
		})
	}
	if len(rows) > 0 {
		r.writeln(r.table(headers, rows))
		if len(n.Connections) > maxRows {
			r.writeln(r.dim(fmt.Sprintf("… and %d more", len(n.Connections)-maxRows)))
		}
	}
}

// Tree renders the full ancestor + descendant hierarchy (the --tree command).
func (r *Renderer) Tree(report *model.Report) {
	if report.Process == nil {
		r.writeln(r.yellow("No process to display."))
		return
	}
	tree := r.renderProcessTree(report)
	r.writeln(r.bold(fmt.Sprintf("PROCESS TREE — PID %d", report.Process.PID)))
	r.writeln(r.hr())
	r.writeln(tree)
}
