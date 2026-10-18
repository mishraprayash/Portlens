package inspector

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
)

type probePorts struct{ l model.Listener }

func (p probePorts) Listeners(context.Context) ([]model.Listener, error) {
	return []model.Listener{p.l}, nil
}
func (p probePorts) ResolvePort(context.Context, uint16, model.Protocol) ([]model.Listener, error) {
	return []model.Listener{p.l}, nil
}

type probeProcs struct{}

func (probeProcs) Info(_ context.Context, pid int32) (*model.ProcessInfo, error) {
	return &model.ProcessInfo{PID: pid, Name: "stub"}, nil
}
func (probeProcs) InfoBasic(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	return probeProcs{}.Info(ctx, pid)
}
func (probeProcs) Exists(context.Context, int32) bool { return true }

// TestProbeIsOptIn verifies that inspecting a port never sends an HTTP request
// to the service unless probing was explicitly enabled.
func TestProbeIsOptIn(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()
	host, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	var port uint16
	for _, c := range portStr {
		port = port*10 + uint16(c-'0')
	}
	plat := &platform.Platform{
		Ports:     probePorts{model.Listener{Protocol: model.ProtocolTCP, Address: host, Port: port, State: "LISTEN", PID: 42}},
		Processes: probeProcs{},
	}

	rep, err := New(plat).InspectDepth(context.Background(), int32(port), model.ProtocolTCP, DepthFast)
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 || rep.HTTPProbe != nil {
		t.Fatalf("probe ran without being enabled (hits=%d)", hits.Load())
	}

	rep, err = New(plat, WithProbe(true)).InspectDepth(context.Background(), int32(port), model.ProtocolTCP, DepthFast)
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() == 0 || rep.HTTPProbe == nil {
		t.Fatal("probe did not run when enabled")
	}
}
