//go:build linux

package platform

import (
	"testing"
	"time"
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
