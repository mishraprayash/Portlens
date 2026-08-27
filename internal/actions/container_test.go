package actions

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// stubContainer is a recordContainerProvider that records container actions.
type recordContainerProvider struct {
	byPort map[uint16]*model.Container
	byPID  map[int32]*model.Container
	stops  int
	kills  int
	starts int
}

func (s *recordContainerProvider) FindByPort(ctx context.Context, port uint16, _ model.Protocol) (*model.Container, error) {
	return s.byPort[port], nil
}

func (s *recordContainerProvider) FindByPorts(context.Context, []uint16, model.Protocol) (map[uint16]*model.Container, error) {
	return nil, nil
}

func (s *recordContainerProvider) FindByPID(ctx context.Context, pid int32) (*model.Container, error) {
	return s.byPID[pid], nil
}

func (s *recordContainerProvider) Stop(context.Context, string, time.Duration) error {
	s.stops++
	return nil
}

func (s *recordContainerProvider) Kill(context.Context, string) error {
	s.kills++
	return nil
}

func (s *recordContainerProvider) Restart(context.Context, string, time.Duration) error {
	s.starts++
	return nil
}

func testContainerManager(containers platform.ContainerProvider, confirm ConfirmFunc) (*Manager, *recordContainerProvider, *bytes.Buffer) {
	rec, _ := containers.(*recordContainerProvider)
	out := &bytes.Buffer{}
	mgr := NewManager(&platform.Platform{Containers: containers}, out, confirm)
	return mgr, rec, out
}

func TestKillContainerGraceful(t *testing.T) {
	c := &model.Container{ID: "abc123", Name: "api-1", Image: "nginx:alpine"}
	mgr, rec, out := testContainerManager(&recordContainerProvider{byPort: map[uint16]*model.Container{8080: c}}, func(string) (bool, error) {
		return true, nil
	})

	report := &model.Report{Port: 8080, Protocol: model.ProtocolTCP, Process: &model.ProcessInfo{PID: 99, Name: "docker-proxy"}}
	if err := mgr.Kill(context.Background(), report, false); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if rec.stops != 1 {
		t.Fatalf("stops = %d, want 1", rec.stops)
	}
	if !strings.Contains(out.String(), "Stopping container api-1") {
		t.Fatalf("output missing stop message: %q", out.String())
	}
}

func TestKillContainerDeclinesWithoutConfirm(t *testing.T) {
	c := &model.Container{ID: "abc123", Name: "api-1"}
	mgr, rec, _ := testContainerManager(&recordContainerProvider{byPort: map[uint16]*model.Container{8080: c}}, func(string) (bool, error) {
		return false, nil
	})
	report := &model.Report{Port: 8080, Process: &model.ProcessInfo{PID: 99}}
	if err := mgr.Kill(context.Background(), report, false); err == nil {
		t.Fatal("expected abort error")
	}
	if rec.stops != 0 {
		t.Fatalf("stops = %d, want 0", rec.stops)
	}
}

func TestKillContainerForceSkipsConfirm(t *testing.T) {
	c := &model.Container{ID: "abc123", Name: "api-1"}
	mgr, rec, _ := testContainerManager(&recordContainerProvider{byPort: map[uint16]*model.Container{8080: c}}, func(string) (bool, error) {
		t.Fatal("confirm should not be called for force kill")
		return false, nil
	})
	report := &model.Report{Port: 8080, Process: &model.ProcessInfo{PID: 99}}
	if err := mgr.Kill(context.Background(), report, true); err != nil {
		t.Fatalf("Kill force: %v", err)
	}
	if rec.kills != 1 {
		t.Fatalf("kills = %d, want 1", rec.kills)
	}
}

func TestKillContainerWhenNoProcessKnown(t *testing.T) {
	c := &model.Container{ID: "abc123", Name: "api-1"}
	mgr, rec, _ := testContainerManager(&recordContainerProvider{byPort: map[uint16]*model.Container{8080: c}}, func(string) (bool, error) {
		return true, nil
	})
	// report.Container is nil and report.Process is nil; the manager re-resolves
	// via FindByPort and still stops the container.
	report := &model.Report{Port: 8080, Protocol: model.ProtocolTCP}
	if err := mgr.Kill(context.Background(), report, false); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if rec.stops != 1 {
		t.Fatalf("stops = %d, want 1", rec.stops)
	}
}

func TestLookupContainerFallsThroughWhenNoMatch(t *testing.T) {
	mgr, _, _ := testContainerManager(&recordContainerProvider{}, nil)
	report := &model.Report{Port: 8080, Protocol: model.ProtocolTCP, Process: &model.ProcessInfo{PID: 99, Name: "node"}}
	if c := mgr.lookupContainer(context.Background(), report); c != nil {
		t.Fatalf("lookupContainer = %+v, want nil", c)
	}
}

func TestKillErrorWithoutProcessOrContainer(t *testing.T) {
	mgr, rec, _ := testContainerManager(&recordContainerProvider{}, nil)
	report := &model.Report{Port: 8080}
	if err := mgr.Kill(context.Background(), report, false); err == nil {
		t.Fatal("expected error for report with no process and no container")
	}
	if rec.stops != 0 && rec.kills != 0 {
		t.Fatalf("no container action should have occurred: %+v", *rec)
	}
}

func TestRestartContainer(t *testing.T) {
	c := &model.Container{ID: "abc123", Name: "api-1"}
	mgr, rec, out := testContainerManager(&recordContainerProvider{byPort: map[uint16]*model.Container{8080: c}}, func(string) (bool, error) {
		return true, nil
	})
	report := &model.Report{Port: 8080, Protocol: model.ProtocolTCP, Process: &model.ProcessInfo{PID: 99}}
	if err := mgr.Restart(context.Background(), report); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if rec.starts != 1 {
		t.Fatalf("restarts = %d, want 1", rec.starts)
	}
	if !strings.Contains(out.String(), "Restarting container api-1") {
		t.Fatalf("output missing restart message: %q", out.String())
	}
}

var _ platform.ContainerProvider = (*recordContainerProvider)(nil)
