package inspector

import (
	"context"
	"net"
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

// BenchmarkSearchByPID exercises SearchByPID against the system process tree.
func BenchmarkSearchByPID(b *testing.B) {
	insp := New(platform.New())
	ctx := context.Background()
	// PID 1 is typically init / launchd or system root.
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := insp.SearchByPID(ctx, 1)
		if err != nil {
			b.Fatalf("SearchByPID: %v", err)
		}
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
