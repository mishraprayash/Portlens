package inspector

import (
	"testing"

	"github.com/portlens/portlens/internal/model"
)

func TestLaunchProcessDirectChildOfShell(t *testing.T) {
	node := &model.ProcessInfo{
		PID:     500,
		Name:    "node",
		Command: "node server.js",
		Cmdline: []string{"node", "server.js"},
	}
	report := &model.Report{
		Port:    3000,
		Process: node,
		Ancestors: []*model.ProcessInfo{
			{PID: 1, Name: "launchd"},
			{PID: 100, Name: "zsh"},
		},
	}

	p := LaunchProcess(report)
	if p == nil {
		t.Fatal("expected non-nil process")
	}
	if p.PID != 500 || p.Name != "node" {
		t.Fatalf("expected node (pid 500), got %s (pid %d)", p.Name, p.PID)
	}
	if cmd := LaunchCommand(report); cmd != "node server.js" {
		t.Fatalf("expected 'node server.js', got %q", cmd)
	}
}

func TestLaunchProcessIntermediaryTool(t *testing.T) {
	pnpm := &model.ProcessInfo{
		PID:     300,
		Name:    "pnpm",
		Command: "pnpm dev",
		Cmdline: []string{"pnpm", "dev"},
	}
	node := &model.ProcessInfo{
		PID:     500,
		Name:    "node",
		Command: "node build/main.js",
	}
	report := &model.Report{
		Port:    3000,
		Process: node,
		Ancestors: []*model.ProcessInfo{
			{PID: 1, Name: "launchd"},
			{PID: 100, Name: "bash"},
			pnpm,
		},
	}

	p := LaunchProcess(report)
	if p == nil {
		t.Fatal("expected non-nil process")
	}
	if p.PID != 300 || p.Name != "pnpm" {
		t.Fatalf("expected pnpm (pid 300), got %s (pid %d)", p.Name, p.PID)
	}
	if cmd := LaunchCommand(report); cmd != "pnpm dev" {
		t.Fatalf("expected 'pnpm dev', got %q", cmd)
	}
}

func TestLaunchProcessSystemDaemon(t *testing.T) {
	postgres := &model.ProcessInfo{
		PID:     999,
		Name:    "postgres",
		Command: "postgres -D /data",
	}
	report := &model.Report{
		Port:    5432,
		Process: postgres,
		Ancestors: []*model.ProcessInfo{
			{PID: 1, Name: "systemd"},
		},
	}

	p := LaunchProcess(report)
	if p != nil {
		t.Fatalf("expected nil for daemon, got %v", p)
	}
	if cmd := LaunchCommand(report); cmd != "" {
		t.Fatalf("expected empty command, got %q", cmd)
	}
}
