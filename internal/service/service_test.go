package service

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
)

type mockInspector struct {
	entries     []model.PortEntry
	reports     map[int32]*model.Report
	pidEntries  map[int32][]model.PortEntry
	nameEntries map[string][]model.PortEntry
	err         error
}

func (m *mockInspector) Inspect(_ context.Context, port int32, proto model.Protocol) (*model.Report, error) {
	return m.InspectDepth(context.Background(), port, proto, inspector.DepthFull)
}

func (m *mockInspector) InspectDepth(_ context.Context, port int32, _ model.Protocol, _ inspector.Depth) (*model.Report, error) {
	if m.err != nil {
		return nil, m.err
	}
	if r, ok := m.reports[port]; ok {
		return r, nil
	}
	return &model.Report{Port: port, Status: "not_listening"}, nil
}

func (m *mockInspector) List(_ context.Context) ([]model.PortEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.entries, nil
}

func (m *mockInspector) SearchByPID(_ context.Context, pid int32) ([]model.PortEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.pidEntries[pid], nil
}

func (m *mockInspector) SearchByName(_ context.Context, query string) ([]model.PortEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nameEntries[query], nil
}

func TestPortServiceList(t *testing.T) {
	mock := &mockInspector{
		entries: []model.PortEntry{
			{Port: 80, Protocol: model.ProtocolTCP},
			{Port: 53, Protocol: model.ProtocolUDP},
		},
	}
	svc := New(WithInspector(mock))

	all, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List() returned %d items, want 2", len(all))
	}

	tcp, err := svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List(onlyTCP) returned error: %v", err)
	}
	if len(tcp) != 1 || tcp[0].Port != 80 {
		t.Errorf("List(onlyTCP) = %v, want port 80 only", tcp)
	}
}

func TestPortServiceInspectValidation(t *testing.T) {
	svc := New()
	if _, err := svc.Inspect(context.Background(), -1, "", inspector.DepthFast); !errors.Is(err, model.ErrInvalidPort) {
		t.Errorf("Inspect(-1) err = %v, want ErrInvalidPort", err)
	}
	if _, err := svc.Inspect(context.Background(), 70000, "", inspector.DepthFast); !errors.Is(err, model.ErrInvalidPort) {
		t.Errorf("Inspect(70000) err = %v, want ErrInvalidPort", err)
	}
}

func TestPortServiceScan(t *testing.T) {
	mock := &mockInspector{
		entries: []model.PortEntry{
			{Port: 3000, Protocol: model.ProtocolTCP},
		},
		reports: map[int32]*model.Report{
			3000: {Port: 3000, Status: "listening", Protocol: model.ProtocolTCP},
		},
	}
	svc := New(WithInspector(mock))

	var progressCalled bool
	found, err := svc.Scan(context.Background(), []int32{3000, 3001, 3002}, model.ProtocolTCP, func(done, total, found int, _ time.Duration) {
		progressCalled = true
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if !progressCalled {
		t.Errorf("Scan expected progress callback to be invoked")
	}
	if len(found) != 1 || found[0].Port != 3000 {
		t.Errorf("Scan found = %v, want [3000]", found)
	}
}

func TestPortServiceFind(t *testing.T) {
	mock := &mockInspector{
		pidEntries: map[int32][]model.PortEntry{
			1234: {{Port: 8080}, {Port: 8081}},
		},
		nameEntries: map[string][]model.PortEntry{
			"node": {{Port: 3000}},
		},
	}
	svc := New(WithInspector(mock))

	ports, err := svc.Find(context.Background(), "", 1234)
	if err != nil || len(ports) != 2 {
		t.Errorf("Find by pid = %v, err = %v", ports, err)
	}

	ports, err = svc.Find(context.Background(), "node", 0)
	if err != nil || len(ports) != 1 || ports[0] != 3000 {
		t.Errorf("Find by name = %v, err = %v", ports, err)
	}

	_, err = svc.Find(context.Background(), "", 0)
	if !errors.Is(err, model.ErrInvalidArguments) {
		t.Errorf("Find without args err = %v, want ErrInvalidArguments", err)
	}
}

func TestPortServiceNextAvailable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot bind tcp listener")
	}
	defer ln.Close()
	boundPort := int32(ln.Addr().(*net.TCPAddr).Port)

	svc := New()
	next, err := svc.NextAvailable(context.Background(), boundPort)
	if err != nil {
		t.Fatalf("NextAvailable err = %v", err)
	}
	if next == boundPort {
		t.Errorf("NextAvailable returned occupied port %d", boundPort)
	}
}
