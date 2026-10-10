package tui

import "sync/atomic"

// singleFlight coalesces background work: at most one run executes at a time,
// and any calls arriving while a run is in progress collapse into exactly one
// queued follow-up. Slow port scans therefore cannot stack on every tick or
// keystroke — the latest request is always served by the next run.
type singleFlight struct {
	flight  atomic.Bool
	pending atomic.Bool
}

// run starts fn in a new goroutine if no run is active, or records that
// another run should happen after the current one finishes.
func (s *singleFlight) run(fn func()) {
	if !s.flight.CompareAndSwap(false, true) {
		s.pending.Store(true)
		return
	}
	go func() {
		for {
			fn()
			if !s.pending.CompareAndSwap(true, false) {
				s.flight.Store(false)
				return
			}
		}
	}()
}
