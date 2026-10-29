package tui

import (
	"testing"
	"time"
)

// TestSingleFlightCoalesces verifies that concurrent requests while a run is
// busy produce exactly one queued follow-up, and that the flight releases
// afterwards so later requests run again.
func TestSingleFlightCoalesces(t *testing.T) {
	var sf singleFlight
	started := make(chan struct{}, 16)
	release := make(chan struct{})
	fn := func() {
		started <- struct{}{}
		<-release
	}

	sf.run(fn)
	<-started // run 1 is now blocked in fn

	for i := 0; i < 5; i++ {
		sf.run(fn)
	}
	select {
	case <-started:
		t.Fatal("a concurrent run started while run 1 was busy")
	case <-time.After(50 * time.Millisecond):
	}

	close(release) // run 1 finishes; the coalesced follow-up runs next
	<-started      // exactly one follow-up

	deadline := time.Now().Add(2 * time.Second)
	for sf.flight.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if sf.flight.Load() {
		t.Fatal("singleFlight never released after the follow-up")
	}

	sf.run(fn) // idle again: a new run must start
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("run after idle did not start")
	}
}
