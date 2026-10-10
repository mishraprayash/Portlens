package actions

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

type stubController struct {
	signaled map[int32]platform.Signal
	alive    map[int32]bool
}

func (s *stubController) Signal(_ context.Context, pid int32, sig platform.Signal) error {
	if s.signaled == nil {
		s.signaled = make(map[int32]platform.Signal)
	}
	s.signaled[pid] = sig
	// Simulate exiting upon SIGTERM
	if sig == platform.SignalTerm || sig == platform.SignalKill {
		delete(s.alive, pid)
	}
	return nil
}

func (s *stubController) IsAlive(_ context.Context, pid int32) bool {
	return s.alive[pid]
}

func TestRestartProcessSuccess(t *testing.T) {
	out := &bytes.Buffer{}
	ctrl := &stubController{
		alive: map[int32]bool{1234: true},
	}
	plat := &platform.Platform{
		Controller: ctrl,
	}
	mgr := NewManager(plat, out, func(prompt string) (bool, error) {
		return true, nil
	})

	var startedArgv []string
	var startedCwd string
	mgr.Starter = func(ctx context.Context, argv []string, cwd string) (int, error) {
		startedArgv = argv
		startedCwd = cwd
		return 5678, nil
	}

	report := &model.Report{
		Port:     3000,
		Protocol: model.ProtocolTCP,
		Process: &model.ProcessInfo{
			PID:     1234,
			Name:    "node",
			Command: "node server.js",
			Cmdline: []string{"node", "server.js"},
			CWD:     "/app",
		},
		Ancestors: []*model.ProcessInfo{
			{Name: "zsh", PID: 100},
		},
	}

	err := mgr.Restart(context.Background(), report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify old process was signaled with SIGTERM
	if ctrl.signaled[1234] != platform.SignalTerm {
		t.Fatalf("expected SIGTERM to old pid 1234, got %v", ctrl.signaled[1234])
	}
	// Verify starter was invoked with correct command and cwd
	if len(startedArgv) != 2 || startedArgv[0] != "node" || startedArgv[1] != "server.js" {
		t.Fatalf("unexpected argv: %v", startedArgv)
	}
	if startedCwd != "/app" {
		t.Fatalf("unexpected cwd: %q", startedCwd)
	}
	if !strings.Contains(out.String(), "Restarted with pid 5678") {
		t.Fatalf("expected restart message in output: %q", out.String())
	}
}

func TestRestartProcessUserAborts(t *testing.T) {
	out := &bytes.Buffer{}
	ctrl := &stubController{
		alive: map[int32]bool{1234: true},
	}
	plat := &platform.Platform{
		Controller: ctrl,
	}
	mgr := NewManager(plat, out, func(prompt string) (bool, error) {
		return false, nil // user says no
	})
	mgr.Starter = func(ctx context.Context, argv []string, cwd string) (int, error) {
		t.Fatal("starter should not be called when user cancels")
		return 0, nil
	}

	report := &model.Report{
		Port: 3000,
		Process: &model.ProcessInfo{
			PID:     1234,
			Name:    "node",
			Command: "node server.js",
			Cmdline: []string{"node", "server.js"},
		},
		Ancestors: []*model.ProcessInfo{{Name: "bash"}},
	}

	err := mgr.Restart(context.Background(), report)
	if err == nil || !strings.Contains(err.Error(), "aborted by user") {
		t.Fatalf("expected aborted error, got: %v", err)
	}
	if len(ctrl.signaled) != 0 {
		t.Fatalf("expected no signals sent, got %v", ctrl.signaled)
	}
}

func TestRestartUnavailable(t *testing.T) {
	mgr := NewManager(&platform.Platform{}, &bytes.Buffer{}, nil)
	report := &model.Report{
		Port: 3000,
		Process: &model.ProcessInfo{
			PID:  1234,
			Name: "systemd",
		},
		// No interactive shell ancestor
		Ancestors: []*model.ProcessInfo{{Name: "launchd", PID: 1}},
	}

	err := mgr.Restart(context.Background(), report)
	if !errors.Is(err, ErrRestartUnavailable) {
		t.Fatalf("expected ErrRestartUnavailable, got %v", err)
	}
}
