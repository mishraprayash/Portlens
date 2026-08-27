package render

import (
	"fmt"

	"github.com/portlens/portlens/internal/model"
)

// History renders previously observed activity for a port.
func (r *Renderer) History(port int32, entries []model.HistoryEntry) {
	r.writeln(r.bold(fmt.Sprintf("PORT %d — HISTORY", port)))
	r.writeln(r.hr())
	if len(entries) == 0 {
		r.writeln(r.dim("No recorded history for this port."))
		r.writeln(r.dim("Run `portlens <port>` to start recording observations locally."))
		return
	}
	for i, e := range entries {
		if i > 0 {
			r.writeln("")
		}
		r.writeln(r.bold(e.ObservedAt.Format("2006-01-02 15:04")))
		rows := [][2]string{
			{"PID", fmt.Sprintf("%d", e.PID)},
			{"Process", e.Process},
			{"Project", e.Project},
			{"Command", e.Command},
			{"Status", e.Status},
		}
		if e.Address != "" {
			rows = append(rows, [2]string{"Address", e.Address})
		}
		r.writeln(r.kv(rows))
	}
}
