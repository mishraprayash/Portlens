package platform

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestProcessTableFreshness(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	p := &processTableTreeProvider{now: func() time.Time { return now }, ttl: time.Second}

	if p.fresh(now) {
		t.Error("fresh on never-built table = true, want false")
	}
	p.built = true
	p.builtAt = now
	if !p.fresh(now.Add(999 * time.Millisecond)) {
		t.Error("fresh within TTL = false, want true")
	}
	if p.fresh(now.Add(time.Second)) {
		t.Error("fresh at TTL boundary = true, want false")
	}
	p.err = errors.New("enumeration failed")
	if p.fresh(now.Add(time.Millisecond)) {
		t.Error("fresh after failed build = true, want false")
	}
}

// TestProcessTableConcurrent exercises concurrent snapshot reads and rebuilds;
// run with -race to verify the swap-in rebuild does not race with readers.
func TestProcessTableConcurrent(t *testing.T) {
	p := newProcessTreeProvider().(*processTableTreeProvider)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 4; j++ {
				if _, err := p.table(); err != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
}
