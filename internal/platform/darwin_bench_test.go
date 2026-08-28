//go:build darwin

package platform

import (
	"context"
	"testing"

	"github.com/portlens/portlens/internal/model"
)

// BenchmarkDarwinResolvePort measures the lsof-based port lookup on macOS — the
// dominant remaining cost on the fast path.
func BenchmarkDarwinResolvePort(b *testing.B) {
	r := darwinPortResolver{}
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ls, err := r.ResolvePort(ctx, 5432, model.ProtocolTCP)
		if err != nil {
			b.Fatal(err)
		}
		if len(ls) == 0 {
			b.Skip("port 5432 not listening; cannot benchmark")
		}
	}
}

// BenchmarkDarwinListeners measures the full listener enumeration used by the
// `portlens` listing command.
func BenchmarkDarwinListeners(b *testing.B) {
	r := darwinPortResolver{}
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := r.Listeners(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
