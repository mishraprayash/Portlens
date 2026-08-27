package inspector

import (
	"context"
	"testing"
	"time"

	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// stubContainerProvider is a deterministic ContainerProvider for tests.
type stubContainerProvider struct {
	byPort map[uint16]*model.Container
	byPID  map[int32]*model.Container
}

func (s *stubContainerProvider) FindByPort(ctx context.Context, port uint16, _ model.Protocol) (*model.Container, error) {
	return s.byPort[port], nil
}

func (s *stubContainerProvider) FindByPorts(ctx context.Context, ports []uint16, protocol model.Protocol) (map[uint16]*model.Container, error) {
	out := map[uint16]*model.Container{}
	for _, p := range ports {
		if c := s.byPort[p]; c != nil {
			out[p] = c
		}
	}
	return out, nil
}

func (s *stubContainerProvider) FindByPID(ctx context.Context, pid int32) (*model.Container, error) {
	return s.byPID[pid], nil
}

func (s *stubContainerProvider) Stop(context.Context, string, time.Duration) error {
	return nil
}

func (s *stubContainerProvider) Kill(context.Context, string) error {
	return nil
}

func (s *stubContainerProvider) Restart(context.Context, string, time.Duration) error {
	return nil
}

func newInspectorWithContainers(byPort map[uint16]*model.Container, byPID map[int32]*model.Container) *Inspector {
	plat := &platform.Platform{Containers: &stubContainerProvider{byPort: byPort, byPID: byPID}}
	return New(plat)
}

func TestAttachContainerPrefersPIDLookup(t *testing.T) {
	pid := &model.Container{ID: "cgroupid", Name: "api-1", Image: "nginx:alpine"}
	byPort := &model.Container{ID: "portid", Name: "from-port", Image: "other"}
	insp := newInspectorWithContainers(map[uint16]*model.Container{8080: byPort}, map[int32]*model.Container{42: pid})

	report := &model.Report{
		Port:     8080,
		Protocol: model.ProtocolTCP,
		Process:  &model.ProcessInfo{PID: 42, Name: "node"},
	}
	insp.attachContainer(context.Background(), report)

	if report.Container == nil || report.Container.Name != "api-1" {
		t.Fatalf("Container = %+v, want api-1 from PID lookup", report.Container)
	}
}

func TestAttachContainerFallsBackToPortLookup(t *testing.T) {
	byPort := &model.Container{ID: "portid", Name: "redis-1", Image: "redis:7"}
	// PID lookup yields no container (host process, e.g. docker-proxy).
	insp := newInspectorWithContainers(map[uint16]*model.Container{6379: byPort}, nil)

	report := &model.Report{
		Port:     6379,
		Protocol: model.ProtocolTCP,
		Process:  &model.ProcessInfo{PID: 99, Name: "docker-proxy"},
	}
	insp.attachContainer(context.Background(), report)

	if report.Container == nil || report.Container.Name != "redis-1" {
		t.Fatalf("Container = %+v, want redis-1 from port lookup", report.Container)
	}
	if len(report.Facts) == 0 {
		t.Fatal("expected a fact about container ownership")
	}
}

func TestAttachContainerWithoutProvider(t *testing.T) {
	insp := New(&platform.Platform{})
	report := &model.Report{Port: 8080, Process: &model.ProcessInfo{PID: 1}}
	insp.attachContainer(context.Background(), report)
	if report.Container != nil {
		t.Fatalf("Container = %+v, want nil", report.Container)
	}
}

func TestAttachContainersBatch(t *testing.T) {
	redis := &model.Container{Name: "redis-1", Image: "redis:7"}
	insp := newInspectorWithContainers(map[uint16]*model.Container{6379: redis}, nil)

	entries := []model.PortEntry{
		{Port: 6379, Process: "docker-proxy"},
		{Port: 8080, Process: "node"},
	}
	insp.attachContainers(context.Background(), entries)

	if entries[0].Container != redis {
		t.Fatalf("entries[0].Container = %+v, want redis", entries[0].Container)
	}
	if entries[1].Container != nil {
		t.Fatalf("entries[1].Container = %+v, want nil", entries[1].Container)
	}
}
