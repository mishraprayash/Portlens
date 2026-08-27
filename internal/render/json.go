package render

import (
	"encoding/json"
	"io"

	"github.com/portlens/portlens/internal/model"
)

// JSON writes a deterministic, pretty-printed JSON representation of the report.
// The schema is documented in docs/json-schema.md.
func JSON(w io.Writer, report *model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(report)
}

// JSONList writes the port listing as a JSON array.
func JSONList(w io.Writer, entries []model.PortEntry) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if entries == nil {
		entries = []model.PortEntry{}
	}
	return enc.Encode(entries)
}

// JSONReports writes multiple inspection reports as a JSON array. It is used
// when several ports are inspected in a single invocation.
func JSONReports(w io.Writer, reports []*model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if reports == nil {
		reports = []*model.Report{}
	}
	return enc.Encode(reports)
}
