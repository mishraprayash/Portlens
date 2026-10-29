package actions

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
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

func restartReport() *model.Report {
	return &model.Report{
		Port: 3000, Protocol: model.ProtocolTCP,
		Process: &model.ProcessInfo{PID: 1234, Name: "node", Command: "node server.js",
			Cmdline: []string{"node", "server.js"}, CWD: "/app"},
		Ancestors: []*model.ProcessInfo{{Name: "zsh", PID: 100}},
	}
}

func TestRestartReportsStarterFailure(t *testing.T) {
	ctrl := &stubController{alive: map[int32]bool{1234: true}}
	mgr := NewManager(&platform.Platform{Controller: ctrl}, &bytes.Buffer{},
		func(string) (bool, error) { return true, nil })
	boom := errors.New("exec: not found")
	mgr.Starter = func(context.Context, []string, string) (int, error) { return 0, boom }
	err := mgr.Restart(context.Background(), restartReport())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if ctrl.signaled[1234] != platform.SignalTerm {
		t.Error("old process should have been stopped before the start attempt")
	}
}

func TestRestartDoesNotStartWhenStopFails(t *testing.T) {
	ctrl := &sticky{} // never exits
	mgr := NewManager(&platform.Platform{Controller: ctrl}, &bytes.Buffer{},
		func(string) (bool, error) { return true, nil })
	mgr.Wait = 150 * time.Millisecond
	started := false
	mgr.Starter = func(context.Context, []string, string) (int, error) { started = true; return 1, nil }
	if err := mgr.Restart(context.Background(), restartReport()); err == nil {
		t.Fatal("expected stop failure")
	}
	if started {
		t.Error("new process started although the old one is still running")
	}
}

func TestRestartAbortedBeforeStopSignalsNothing(t *testing.T) {
	ctrl := &sticky{}
	mgr := NewManager(&platform.Platform{Controller: ctrl}, &bytes.Buffer{},
		func(string) (bool, error) { return false, nil })
	mgr.Starter = func(context.Context, []string, string) (int, error) { t.Fatal("started"); return 0, nil }
	if err := mgr.Restart(context.Background(), restartReport()); err == nil {
		t.Fatal("expected abort")
	}
	if len(ctrl.order) != 0 {
		t.Errorf("signals sent: %v", ctrl.order)
	}
}

func TestDefaultProcessStarterRunsDetached(t *testing.T) {
	pid, err := defaultProcessStarter(context.Background(), []string{"true"}, t.TempDir())
	if err != nil || pid <= 0 {
		t.Fatalf("pid %d, err %v", pid, err)
	}
	if _, err := defaultProcessStarter(context.Background(), []string{"/definitely/not/a/binary"}, ""); err == nil {
		t.Error("expected start error for missing binary")
	}
}
