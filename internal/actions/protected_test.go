package actions

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
)

func TestTerminateTreeRefusesProtectedPIDs(t *testing.T) {
	for _, pid := range []int32{-1, 0, 1, int32(os.Getpid())} {
		ctrl := &stubController{alive: map[int32]bool{pid: true}}
		mgr := NewManager(&platform.Platform{Controller: ctrl}, &bytes.Buffer{}, nil)
		err := mgr.TerminateTree(context.Background(), pid, true)
		if !errors.Is(err, ErrProtectedProcess) {
			t.Errorf("pid %d: err = %v, want ErrProtectedProcess", pid, err)
		}
		if len(ctrl.signaled) != 0 {
			t.Errorf("pid %d: signals sent: %v", pid, ctrl.signaled)
		}
	}
}

func TestPlanKillDoesNotSignal(t *testing.T) {
	ctrl := &stubController{alive: map[int32]bool{4242: true}}
	mgr := NewManager(&platform.Platform{Controller: ctrl}, &bytes.Buffer{}, nil)
	rep := &model.Report{Port: 3000, Process: &model.ProcessInfo{PID: 4242, Name: "node"}}
	plan, err := mgr.PlanKill(context.Background(), rep, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "SIGTERM") || !strings.Contains(plan, "4242") {
		t.Errorf("plan = %q", plan)
	}
	if len(ctrl.signaled) != 0 {
		t.Errorf("dry run signaled: %v", ctrl.signaled)
	}
	if _, err := mgr.PlanKill(context.Background(), &model.Report{Process: &model.ProcessInfo{PID: 1}}, true); !errors.Is(err, ErrProtectedProcess) {
		t.Errorf("PID 1 err = %v", err)
	}
}
