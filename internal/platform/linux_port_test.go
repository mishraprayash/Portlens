//go:build linux

package platform

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

func TestSharedInodeMapCachesWithinTTL(t *testing.T) {
	builds := 0
	m := newSharedInodeMap(50*time.Millisecond, func() map[uint64]int32 {
		builds++
		return map[uint64]int32{}
	})

	m.get()
	m.get()
	if builds != 1 {
		t.Fatalf("expected exactly 1 build within the TTL, got %d", builds)
	}

	time.Sleep(100 * time.Millisecond)
	m.get()
	if builds != 2 {
		t.Fatalf("expected a rebuild after the TTL elapsed, got %d builds", builds)
	}
}

func TestSharedInodeMapRefreshForcesRebuild(t *testing.T) {
	builds := 0
	m := newSharedInodeMap(time.Hour, func() map[uint64]int32 {
		builds++
		return map[uint64]int32{uint64(builds): 1}
	})

	m.get()
	if builds != 1 {
		t.Fatalf("builds after get = %d, want 1", builds)
	}
	// Within the TTL get would not rebuild; refresh must.
	m.refresh()
	if builds != 2 {
		t.Fatalf("builds after refresh = %d, want 2", builds)
	}
	// The refreshed map is published, so a subsequent get serves it.
	m.get()
	if builds != 2 {
		t.Fatalf("builds after get on refreshed map = %d, want 2", builds)
	}
	if _, ok := m.get()[2]; !ok {
		t.Error("refreshed entry missing from published map")
	}
}

func TestResolvePortRefreshesStaleInodeMap(t *testing.T) {
	// Warm the cache while no listener exists; with a 1-hour TTL, get() will
	// keep returning that stale map, so only the refresh-on-miss path can
	// attribute the listener below.
	shared := newSharedInodeMap(time.Hour, socketInodeMap)
	shared.get()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	r := &linuxPortResolver{inodes: shared}
	ls, err := r.ResolvePort(context.Background(), uint16(port), model.ProtocolTCP)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) == 0 {
		t.Fatal("expected the new listener to resolve")
	}
	if ls[0].PID == 0 {
		t.Error("PID = 0: stale inode map was not refreshed on miss")
	}
	if want := int32(os.Getpid()); ls[0].PID != want {
		t.Errorf("PID = %d, want %d (this process)", ls[0].PID, want)
	}
}
