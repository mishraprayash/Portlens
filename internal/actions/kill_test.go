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

// stubTree serves a fixed descendant tree.
type stubTree struct{ tree *model.ProcessTree }

func (s stubTree) Ancestors(context.Context, int32) ([]*model.ProcessInfo, error) { return nil, nil }
func (s stubTree) Children(context.Context, int32) ([]*model.ProcessInfo, error)  { return nil, nil }
func (s stubTree) Descendants(context.Context, int32) (*model.ProcessTree, error) {
	return s.tree, nil
}

// sticky is a controller whose processes ignore signals (they never exit) and
// that records the order signals were sent in.
type sticky struct {
	order  []int32
	sigs   []platform.Signal
	fail   error
	exited map[int32]bool
}

func (s *sticky) Signal(_ context.Context, pid int32, sig platform.Signal) error {
	s.order = append(s.order, pid)
	s.sigs = append(s.sigs, sig)
	return s.fail
}
func (s *sticky) IsAlive(_ context.Context, pid int32) bool { return !s.exited[pid] }

func proc(pid int32) model.ProcessInfo { return model.ProcessInfo{PID: pid, Name: "p"} }

// 100 -> (200 -> 300), 400
func sampleTree() *model.ProcessTree {
	return &model.ProcessTree{Process: proc(100), Children: []*model.ProcessTree{
		{Process: proc(200), Children: []*model.ProcessTree{{Process: proc(300)}}},
		{Process: proc(400)},
	}}
}

func TestTreePIDsDeepestFirst(t *testing.T) {
	got := TreePIDs(sampleTree())
	want := []int32{300, 200, 400, 100}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if TreePIDs(nil) != nil {
		t.Error("nil tree should yield nil")
	}
}

func killReport() *model.Report {
	return &model.Report{Port: 3000, Process: &model.ProcessInfo{PID: 100, Name: "node"}}
}

func TestKillGracefulSignalsWholeTreeAndReportsExit(t *testing.T) {
	ctrl := &sticky{exited: map[int32]bool{100: true}} // root exits right away
	out := &bytes.Buffer{}
	mgr := NewManager(&platform.Platform{Controller: ctrl, Tree: stubTree{sampleTree()}}, out,
		func(string) (bool, error) { return true, nil })
	if err := mgr.Kill(context.Background(), killReport(), false); err != nil {
		t.Fatal(err)
	}
	if len(ctrl.order) != 4 || ctrl.order[0] != 300 || ctrl.order[3] != 100 {
		t.Errorf("signal order = %v, want deepest first", ctrl.order)
	}
	for _, s := range ctrl.sigs {
		if s != platform.SignalTerm {
			t.Errorf("graceful kill sent %v", s)
		}
	}
	if !strings.Contains(out.String(), "3 descendant(s)") || !strings.Contains(out.String(), "exited gracefully") {
		t.Errorf("output = %q", out.String())
	}
}

func TestKillDeclinedSendsNothing(t *testing.T) {
	ctrl := &sticky{}
	mgr := NewManager(&platform.Platform{Controller: ctrl, Tree: stubTree{sampleTree()}}, &bytes.Buffer{},
		func(string) (bool, error) { return false, nil })
	if err := mgr.Kill(context.Background(), killReport(), false); err == nil {
		t.Fatal("expected abort error")
	}
	if len(ctrl.order) != 0 {
		t.Errorf("signals sent despite decline: %v", ctrl.order)
	}
}

func TestKillConfirmErrorPropagates(t *testing.T) {
	boom := errors.New("tty gone")
	mgr := NewManager(&platform.Platform{Controller: &sticky{}, Tree: stubTree{}}, &bytes.Buffer{},
		func(string) (bool, error) { return false, boom })
	if err := mgr.Kill(context.Background(), killReport(), false); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestKillForceSkipsPromptAndSendsSIGKILL(t *testing.T) {
	ctrl := &sticky{exited: map[int32]bool{100: true}}
	prompted := false
	mgr := NewManager(&platform.Platform{Controller: ctrl, Tree: stubTree{}}, &bytes.Buffer{},
		func(string) (bool, error) { prompted = true; return true, nil })
	if err := mgr.Kill(context.Background(), killReport(), true); err != nil {
		t.Fatal(err)
	}
	if prompted {
		t.Error("force kill must not prompt")
	}
	if len(ctrl.sigs) != 1 || ctrl.sigs[0] != platform.SignalKill {
		t.Errorf("signals = %v", ctrl.sigs)
	}
}

func TestKillReportsStillRunning(t *testing.T) {
	ctrl := &sticky{} // never exits
	mgr := NewManager(&platform.Platform{Controller: ctrl, Tree: stubTree{}}, &bytes.Buffer{},
		func(string) (bool, error) { return true, nil })
	mgr.Wait = 200 * time.Millisecond
	err := mgr.Kill(context.Background(), killReport(), false)
	var still *ErrStillRunning
	if !errors.As(err, &still) || still.PID != 100 {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(still.Error(), "100") {
		t.Errorf("message = %q", still.Error())
	}
}

func TestKillPropagatesSignalError(t *testing.T) {
	perm := errors.New("operation not permitted")
	mgr := NewManager(&platform.Platform{Controller: &sticky{fail: perm}, Tree: stubTree{}}, &bytes.Buffer{},
		func(string) (bool, error) { return true, nil })
	if err := mgr.Kill(context.Background(), killReport(), false); !errors.Is(err, perm) {
		t.Fatalf("err = %v", err)
	}
}

func TestKillNilReport(t *testing.T) {
	mgr := NewManager(&platform.Platform{}, &bytes.Buffer{}, nil)
	if err := mgr.Kill(context.Background(), nil, false); err == nil {
		t.Fatal("expected error")
	}
}

func TestKillSkipsProtectedDescendant(t *testing.T) {
	tree := &model.ProcessTree{Process: proc(100), Children: []*model.ProcessTree{{Process: proc(1)}}}
	ctrl := &sticky{exited: map[int32]bool{100: true}}
	mgr := NewManager(&platform.Platform{Controller: ctrl, Tree: stubTree{tree}}, &bytes.Buffer{},
		func(string) (bool, error) { return true, nil })
	if err := mgr.Kill(context.Background(), killReport(), true); err != nil {
		t.Fatal(err)
	}
	for _, p := range ctrl.order {
		if p == 1 {
			t.Fatal("PID 1 was signaled")
		}
	}
}

func TestPlanKillTreeAndContainer(t *testing.T) {
	mgr := NewManager(&platform.Platform{Controller: &sticky{}, Tree: stubTree{sampleTree()}}, &bytes.Buffer{}, nil)
	plan, err := mgr.PlanKill(context.Background(), killReport(), true)
	if err != nil || !strings.Contains(plan, "SIGKILL") || !strings.Contains(plan, "4 process(es)") {
		t.Fatalf("plan = %q, err = %v", plan, err)
	}
	if _, err := mgr.PlanKill(context.Background(), nil, false); err == nil {
		t.Error("nil report should error")
	}
	if _, err := mgr.PlanKill(context.Background(), &model.Report{Port: 1}, false); err == nil {
		t.Error("no process should error")
	}
	c := &model.Container{ID: "abc", Name: "api"}
	cm := NewManager(&platform.Platform{Containers: &recordContainerProvider{}}, &bytes.Buffer{}, nil)
	plan, _ = cm.PlanKill(context.Background(), &model.Report{Container: c}, false)
	if !strings.Contains(plan, "stop container api") {
		t.Errorf("container plan = %q", plan)
	}
	plan, _ = cm.PlanKill(context.Background(), &model.Report{Container: c}, true)
	if !strings.Contains(plan, "force-stop container api") {
		t.Errorf("container force plan = %q", plan)
	}
}

type stubClipboard struct{ text string }

func (s *stubClipboard) Copy(_ context.Context, text string) error { s.text = text; return nil }

func TestCopyDelegates(t *testing.T) {
	cb := &stubClipboard{}
	mgr := NewManager(&platform.Platform{Clipboard: cb}, &bytes.Buffer{}, nil)
	if err := mgr.Copy(context.Background(), "3000"); err != nil || cb.text != "3000" {
		t.Fatalf("copied %q, err %v", cb.text, err)
	}
}

func TestLooksLikeHTTP(t *testing.T) {
	cases := []struct {
		name string
		r    *model.Report
		want bool
	}{
		{"framework", &model.Report{Project: &model.ProjectInfo{Framework: "flask"}}, true},
		{"runtime", &model.Report{Project: &model.ProjectInfo{Runtime: "node"}}, true},
		{"dev command", &model.Report{Process: &model.ProcessInfo{Command: "pnpm DEV"}}, true},
		{"database", &model.Report{Process: &model.ProcessInfo{Command: "postgres -D /data"}}, false},
		{"empty", &model.Report{}, false},
	}
	for _, c := range cases {
		if got := looksLikeHTTP(c.r); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLocalURLIPv6(t *testing.T) {
	if got := LocalURL(&model.Report{Address: "fe80::1", Port: 8080}); got != "http://[fe80::1]:8080" {
		t.Errorf("got %q", got)
	}
}
