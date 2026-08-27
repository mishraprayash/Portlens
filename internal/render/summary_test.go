package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/portlens/portlens/internal/model"
)

func TestSummaryListening(t *testing.T) {
	report := &model.Report{
		Port:     3000,
		Protocol: model.ProtocolTCP,
		Status:   "listening",
		Address:  "127.0.0.1",
		Process:  &model.ProcessInfo{PID: 48231, Name: "node", Command: "pnpm dev"},
		Project:  &model.ProjectInfo{Name: "orbit-backend", Runtime: "node", Detected: true},
		Exposure: &model.Exposure{Worst: model.RiskLow},
	}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Summary(report)

	out := buf.String()
	for _, want := range []string{"PORT 3000", "LISTENING", "127.0.0.1", "node (pid 48231)", "pnpm dev", "orbit-backend", "LOW RISK"} {
		if !strings.Contains(out, want) {
			t.Errorf("Summary output missing %q:\n%s", want, out)
		}
	}
	for _, notWant := range []string{"PROCESS TREE", "NETWORK", "INTERPRETATION", "ACTIONS"} {
		if strings.Contains(out, notWant) {
			t.Errorf("Summary should not contain %q:\n%s", notWant, out)
		}
	}
}

func TestSummaryNotListening(t *testing.T) {
	report := &model.Report{Port: 9999, Status: "not_listening"}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Summary(report)

	out := buf.String()
	if !strings.Contains(out, "PORT 9999") || !strings.Contains(out, "No process is currently listening") {
		t.Errorf("Summary not-listening output:\n%s", out)
	}
}

func TestReportVerbose(t *testing.T) {
	report := &model.Report{
		Port:     3000,
		Protocol: model.ProtocolTCP,
		Status:   "listening",
		Address:  "127.0.0.1",
		Process:  &model.ProcessInfo{PID: 1, Name: "node"},
	}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Report(report)

	out := buf.String()
	if !strings.Contains(out, "ACTIONS") {
		t.Errorf("verbose Report should include sections:\n%s", out)
	}
}
