package render

import (
	"bytes"
	"io"
	"testing"

	"github.com/portlens/portlens/internal/model"
)

func benchReport() *model.Report {
	return &model.Report{
		Port:     5432,
		Protocol: model.ProtocolTCP,
		Status:   "listening",
		Address:  "127.0.0.1",
		Service:  "PostgreSQL",
		Process: &model.ProcessInfo{
			PID: 946, PPID: 1, Name: "postgres",
			Exe:     "/opt/homebrew/opt/postgresql@14/bin/postgres",
			Command: "postgres -D /opt/homebrew/var/postgresql@14",
			CWD:     "/opt/homebrew/var/postgresql@14",
			User:    "prayashmishra",
		},
		Origin: model.OriginUser,
		Project: &model.ProjectInfo{
			Name: "brew", Directory: "/opt/homebrew", Runtime: "postgres", Detected: true,
		},
		Network: &model.NetworkInfo{
			Listeners: []model.Listener{
				{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 5432, State: "LISTEN", PID: 946, Process: "postgres"},
			},
			Summary: model.NetworkSummary{Total: 1, ByState: map[string]int{"ESTABLISHED": 1}, LocalOnly: true},
		},
		Exposure:       &model.Exposure{BoundLocalhost: true, Findings: []model.Finding{{Level: model.RiskLow, Reason: "Bound only to loopback"}}, Worst: model.RiskLow},
		Interpretation: "PostgreSQL process",
		Facts:          []string{"Process 946 (postgres) is listening on 127.0.0.1:5432 over tcp"},
	}
}

func BenchmarkSummary(b *testing.B) {
	r := New(io.Discard, false)
	report := benchReport()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Summary(report)
	}
}

func BenchmarkReportVerbose(b *testing.B) {
	r := New(io.Discard, false)
	report := benchReport()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Report(report)
	}
}

func BenchmarkJSONOutput(b *testing.B) {
	report := benchReport()
	var buf bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = JSON(&buf, report)
	}
}

func BenchmarkListTable(b *testing.B) {
	entries := []model.PortEntry{
		{Port: 5432, Protocol: model.ProtocolTCP, Process: "postgres", Service: "PostgreSQL", Project: "brew", Runtime: "postgres", Address: "127.0.0.1", Status: "LISTEN", Origin: model.OriginUser},
		{Port: 6379, Protocol: model.ProtocolTCP, Process: "redis-server", Service: "Redis", Project: "brew", Runtime: "redis", Address: "127.0.0.1", Status: "LISTEN", Origin: model.OriginUser},
		{Port: 88, Protocol: model.ProtocolTCP, Process: "kdc", Service: "Kerberos", Address: "0.0.0.0", Status: "LISTEN", Origin: model.OriginSystem},
	}
	r := New(io.Discard, false)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.List(entries, ListOptions{})
	}
}
