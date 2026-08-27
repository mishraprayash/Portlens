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

func TestSummaryWithContainer(t *testing.T) {
	report := &model.Report{
		Port:     8080,
		Protocol: model.ProtocolTCP,
		Status:   "listening",
		Address:  "127.0.0.1",
		Process:  &model.ProcessInfo{PID: 48231, Name: "nginx", Command: "nginx -g daemon off;"},
		Container: &model.Container{
			ID:     "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			Name:   "api-1",
			Image:  "nginx:alpine",
			Status: "running",
		},
		Exposure: &model.Exposure{Worst: model.RiskWarning},
	}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Summary(report)

	out := buf.String()
	for _, want := range []string{"PORT 8080", "api-1 (nginx:alpine)", "nginx (pid 48231)"} {
		if !strings.Contains(out, want) {
			t.Errorf("Summary output missing %q:\n%s", want, out)
		}
	}
}

func TestReportWithContainerSection(t *testing.T) {
	report := &model.Report{
		Port:     6379,
		Protocol: model.ProtocolTCP,
		Status:   "listening",
		Address:  "127.0.0.1",
		Process:  &model.ProcessInfo{PID: 48231, Name: "redis-server"},
		Container: &model.Container{
			ID:             "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Name:           "redis-1",
			Image:          "redis:7",
			Status:         "running",
			ComposeProject: "orbit",
			ComposeService: "redis",
		},
	}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Report(report)

	out := buf.String()
	for _, want := range []string{"CONTAINER", "Name", "redis-1", "bbbbbbbbbbbb", "redis:7", "Compose Project", "orbit", "Compose Service", "redis"} {
		if !strings.Contains(out, want) {
			t.Errorf("Report output missing %q:\n%s", want, out)
		}
	}
}

func TestListShowsContainerColumnWhenPresent(t *testing.T) {
	entries := []model.PortEntry{
		{Port: 6379, Process: "redis-server", Project: "-", Runtime: "-", Address: "127.0.0.1", Status: "LISTEN",
			Container: &model.Container{Name: "redis-1", Image: "redis:7"}},
		{Port: 3000, Process: "node", Project: "orbit", Runtime: "node", Address: "127.0.0.1", Status: "LISTEN"},
	}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.List(entries, ListOptions{})

	out := buf.String()
	for _, want := range []string{"CONTAINER", "redis-1", "3000"} {
		if !strings.Contains(out, want) {
			t.Errorf("List output missing %q:\n%s", want, out)
		}
	}
}

func TestListOmitsContainerColumnWhenAbsent(t *testing.T) {
	entries := []model.PortEntry{
		{Port: 3000, Process: "node", Project: "orbit", Runtime: "node", Address: "127.0.0.1", Status: "LISTEN"},
	}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.List(entries, ListOptions{})

	if strings.Contains(buf.String(), "CONTAINER") {
		t.Errorf("List should omit CONTAINER column when no entry has a container:\n%s", buf.String())
	}
}

func TestListFilterMatchesContainer(t *testing.T) {
	entries := []model.PortEntry{
		{Port: 6379, Process: "redis-server", Address: "127.0.0.1", Status: "LISTEN",
			Container: &model.Container{Name: "cache-1", Image: "redis:7"}},
		{Port: 3000, Process: "node", Address: "127.0.0.1", Status: "LISTEN"},
	}
	var buf bytes.Buffer
	r := New(&buf, false)
	r.List(entries, ListOptions{Filter: "cache-1"})

	out := buf.String()
	if !strings.Contains(out, "6379") || strings.Contains(out, "3000") {
		t.Errorf("filter by container name should keep only the matching row:\n%s", out)
	}
}
