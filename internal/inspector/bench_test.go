package inspector

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// benchPort returns a port that is very likely listening during the benchmark
// so the full Inspect path is exercised. On CI the listener is owned by the
// test itself, guaranteeing a stable owner.
func benchPort(tb testing.TB) int32 {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Skipf("cannot bind a listener: %v", err)
	}
	tb.Cleanup(func() { _ = ln.Close() })
	return int32(ln.Addr().(*net.TCPAddr).Port)
}

// BenchmarkInspectPort exercises the full single-port inspection pipeline
// against a live listener owned by the test process.
func BenchmarkInspectPort(b *testing.B) {
	port := benchPort(b)
	insp := New(platform.New())
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		report, err := insp.InspectDepth(ctx, port, model.ProtocolTCP, DepthFull)
		if err != nil {
			b.Fatalf("Inspect: %v", err)
		}
		if report.Status != "listening" {
			b.Fatalf("status = %s", report.Status)
		}
	}
}

func BenchmarkList(b *testing.B) {
	insp := New(platform.New())
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := insp.List(ctx)
		if err != nil {
			b.Fatalf("List: %v", err)
		}
	}
}

func BenchmarkProcessInfos(b *testing.B) {
	insp := New(platform.New())
	ctx := context.Background()
	pid := int32(os.Getpid())
	listeners := []model.Listener{
		{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 8080, PID: pid},
		{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 8081, PID: pid},
		{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 8082, PID: 1},
		{Protocol: model.ProtocolUDP, Address: "0.0.0.0", Port: 53, PID: 1},
	}
	if live, err := insp.Platform.Ports.Listeners(ctx); err == nil && len(live) > 0 {
		listeners = append(listeners, live...)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = insp.processInfos(ctx, listeners)
	}
}

// BenchmarkInspectPortFast measures the default fast path that `portlens <port>`
// uses: ownership, minimal process info, project, exposure — no process tree or
// network connections.
func BenchmarkInspectPortFast(b *testing.B) {
	port := benchPort(b)
	insp := New(platform.New())
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		report, err := insp.InspectDepth(ctx, port, model.ProtocolTCP, DepthFast)
		if err != nil {
			b.Fatalf("Inspect: %v", err)
		}
		if report.Status != "listening" {
			b.Fatalf("status = %s", report.Status)
		}
	}
}
