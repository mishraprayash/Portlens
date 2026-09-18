package cmd

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

func TestRunNextDefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runNext(context.Background(), nil, &stdout, &stderr)
	if code != exitcode.Success {
		t.Fatalf("runNext returned %d, want 0 (stderr: %s)", code, stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	p, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("runNext output %q is not a number", out)
	}
	if p < 3000 || p > 65535 {
		t.Errorf("expected port in 3000..65535, got %d", p)
	}
}

func TestRunNextStartPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runNext(context.Background(), []string{"8500"}, &stdout, &stderr)
	if code != exitcode.Success {
		t.Fatalf("runNext returned %d, want 0 (stderr: %s)", code, stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	p, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("runNext output %q is not a number", out)
	}
	if p < 8500 || p > 65535 {
		t.Errorf("expected port >= 8500, got %d", p)
	}
}

func TestRunNextInvalidPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runNext(context.Background(), []string{"99999"}, &stdout, &stderr)
	if code != exitcode.InvalidArguments {
		t.Fatalf("runNext returned %d, want %d", code, exitcode.InvalidArguments)
	}
}

type mockPortResolver struct {
	listeners []model.Listener
	err       error
}

func (m *mockPortResolver) Listeners(ctx context.Context) ([]model.Listener, error) {
	return m.listeners, m.err
}

func (m *mockPortResolver) ResolvePort(ctx context.Context, port uint16, protocol model.Protocol) ([]model.Listener, error) {
	return nil, nil
}

func TestGetInUsePorts(t *testing.T) {
	ctx := context.Background()

	// Case 1: nil inspector
	if inUse := getInUsePorts(ctx, nil, model.ProtocolTCP); len(inUse) != 0 {
		t.Errorf("expected empty map for nil inspector, got %v", inUse)
	}

	// Case 2: nil Platform
	inspNilPlat := &inspector.Inspector{}
	if inUse := getInUsePorts(ctx, inspNilPlat, model.ProtocolTCP); len(inUse) != 0 {
		t.Errorf("expected empty map for nil Platform, got %v", inUse)
	}

	// Case 3: nil Ports
	inspNilPorts := &inspector.Inspector{Platform: &platform.Platform{}}
	if inUse := getInUsePorts(ctx, inspNilPorts, model.ProtocolTCP); len(inUse) != 0 {
		t.Errorf("expected empty map for nil Ports, got %v", inUse)
	}

	// Case 4: Listeners error
	inspErr := &inspector.Inspector{
		Platform: &platform.Platform{
			Ports: &mockPortResolver{err: errors.New("listeners error")},
		},
	}
	if inUse := getInUsePorts(ctx, inspErr, model.ProtocolTCP); len(inUse) != 0 {
		t.Errorf("expected empty map on Listeners error, got %v", inUse)
	}

	// Case 5: Success & Protocol filtering
	mockRes := &mockPortResolver{
		listeners: []model.Listener{
			{Port: 8080, Protocol: model.ProtocolTCP},
			{Port: 9090, Protocol: model.ProtocolUDP},
		},
	}
	inspValid := &inspector.Inspector{
		Platform: &platform.Platform{
			Ports: mockRes,
		},
	}

	// Filter TCP
	tcpInUse := getInUsePorts(ctx, inspValid, model.ProtocolTCP)
	if !tcpInUse[8080] || tcpInUse[9090] {
		t.Errorf("expected only 8080 for TCP, got %v", tcpInUse)
	}

	// Filter UDP
	udpInUse := getInUsePorts(ctx, inspValid, model.ProtocolUDP)
	if udpInUse[8080] || !udpInUse[9090] {
		t.Errorf("expected only 9090 for UDP, got %v", udpInUse)
	}

	// Filter empty protocol (matches all)
	allInUse := getInUsePorts(ctx, inspValid, "")
	if !allInUse[8080] || !allInUse[9090] {
		t.Errorf("expected both 8080 and 9090 for empty proto, got %v", allInUse)
	}
}
