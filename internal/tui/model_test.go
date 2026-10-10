package tui

import (
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

func sampleEntries() []model.PortEntry {
	return []model.PortEntry{
		{Port: 80, Protocol: model.ProtocolTCP, Process: "nginx", PID: 100, Service: "HTTP"},
		{Port: 443, Protocol: model.ProtocolTCP, Process: "nginx", PID: 100, Service: "HTTPS"},
		{Port: 3000, Protocol: model.ProtocolTCP, Process: "node", PID: 1024, Service: "NodeJS", Project: "my-web-app"},
		{Port: 5432, Protocol: model.ProtocolTCP, Process: "postgres", PID: 200, Service: "PostgreSQL"},
		{Port: 6379, Protocol: model.ProtocolTCP, Process: "redis-server", PID: 300, Service: "Redis"},
		{Port: 5353, Protocol: model.ProtocolUDP, Process: "mDNSResponder", PID: 50, Service: "mDNS"},
	}
}

func TestModelNavigation(t *testing.T) {
	m := NewModel(false)
	m.SetEntries(sampleEntries())

	if len(m.FilteredIndices) != 6 {
		t.Fatalf("expected 6 entries, got %d", len(m.FilteredIndices))
	}

	// Initial selection
	if m.SelectedIdx != 0 {
		t.Fatalf("expected selected 0, got %d", m.SelectedIdx)
	}
	if m.SelectedEntry().Port != 80 {
		t.Errorf("expected port 80, got %d", m.SelectedEntry().Port)
	}

	// MoveDown
	m.MoveDown()
	if m.SelectedIdx != 1 || m.SelectedEntry().Port != 443 {
		t.Errorf("expected port 443 at idx 1, got %v", m.SelectedEntry())
	}

	// End
	m.End()
	if m.SelectedIdx != 5 || m.SelectedEntry().Port != 5353 {
		t.Errorf("expected port 5353 at idx 5, got %v", m.SelectedEntry())
	}

	// MoveDown at bottom should clamp
	m.MoveDown()
	if m.SelectedIdx != 5 {
		t.Errorf("expected clamped at 5, got %d", m.SelectedIdx)
	}

	// MoveUp
	m.MoveUp()
	if m.SelectedIdx != 4 {
		t.Errorf("expected idx 4, got %d", m.SelectedIdx)
	}

	// Home
	m.Home()
	if m.SelectedIdx != 0 {
		t.Errorf("expected idx 0, got %d", m.SelectedIdx)
	}

	// MoveUp at top should clamp
	m.MoveUp()
	if m.SelectedIdx != 0 {
		t.Errorf("expected clamped at 0, got %d", m.SelectedIdx)
	}

	// PageDown and PageUp
	m.PageDown(3)
	if m.SelectedIdx != 3 {
		t.Errorf("expected idx 3, got %d", m.SelectedIdx)
	}
	m.PageUp(2)
	if m.SelectedIdx != 1 {
		t.Errorf("expected idx 1, got %d", m.SelectedIdx)
	}
}

func TestModelFiltering(t *testing.T) {
	m := NewModel(false)
	m.SetEntries(sampleEntries())

	// Filter for "node"
	m.SetFilter("node")
	if len(m.FilteredIndices) != 1 {
		t.Fatalf("expected 1 result for 'node', got %d", len(m.FilteredIndices))
	}
	if m.SelectedEntry().Port != 3000 {
		t.Errorf("expected port 3000, got %d", m.SelectedEntry().Port)
	}

	// Filter for "5432"
	m.SetFilter("5432")
	if len(m.FilteredIndices) != 1 || m.SelectedEntry().Port != 5432 {
		t.Errorf("expected port 5432, got %v", m.SelectedEntry())
	}

	// Filter for nonexistent
	m.SetFilter("nonexistent")
	if len(m.FilteredIndices) != 0 {
		t.Errorf("expected 0 results, got %d", len(m.FilteredIndices))
	}
	if m.SelectedEntry() != nil {
		t.Errorf("expected nil selected entry, got %v", m.SelectedEntry())
	}

	// Clear filter
	m.SetFilter("")
	if len(m.FilteredIndices) != 6 {
		t.Errorf("expected 6 results on cleared filter, got %d", len(m.FilteredIndices))
	}
}

func TestModelTCPOnly(t *testing.T) {
	m := NewModel(true)
	m.SetEntries(sampleEntries())

	// Should exclude UDP (5353)
	if len(m.FilteredIndices) != 5 {
		t.Errorf("expected 5 TCP entries, got %d", len(m.FilteredIndices))
	}
	for _, idx := range m.FilteredIndices {
		if m.Entries[idx].Protocol.Normalize() != model.ProtocolTCP {
			t.Errorf("found non-TCP entry: %v", m.Entries[idx])
		}
	}
}

func TestModelSelectionPreservation(t *testing.T) {
	m := NewModel(false)
	m.SetEntries(sampleEntries())

	// Select port 5432 (idx 3)
	m.SelectedIdx = 3
	if m.SelectedEntry().Port != 5432 {
		t.Fatalf("expected port 5432 selected")
	}

	// New list where 5432 moved to index 0
	updated := []model.PortEntry{
		{Port: 5432, Protocol: model.ProtocolTCP, Process: "postgres", PID: 200},
		{Port: 80, Protocol: model.ProtocolTCP, Process: "nginx", PID: 100},
	}
	m.SetEntries(updated)

	if m.SelectedEntry() == nil || m.SelectedEntry().Port != 5432 {
		t.Errorf("expected port 5432 still selected after re-ordering, got %v", m.SelectedEntry())
	}
	if m.SelectedIdx != 0 {
		t.Errorf("expected selected idx 0, got %d", m.SelectedIdx)
	}
}

func TestModelScrolling(t *testing.T) {
	m := NewModel(false)
	entries := make([]model.PortEntry, 30)
	for i := 0; i < 30; i++ {
		entries[i] = model.PortEntry{Port: int32(8000 + i), Protocol: model.ProtocolTCP}
	}
	m.SetEntries(entries)

	visibleRows := 10
	m.EnsureVisible(visibleRows)
	if m.ScrollOffset != 0 {
		t.Errorf("expected scroll offset 0, got %d", m.ScrollOffset)
	}

	// Move down past visible area
	m.SelectedIdx = 15
	m.EnsureVisible(visibleRows)
	if m.ScrollOffset < 6 {
		t.Errorf("expected scroll offset >= 6, got %d", m.ScrollOffset)
	}

	// Move back to top
	m.Home()
	m.EnsureVisible(visibleRows)
	if m.ScrollOffset != 0 {
		t.Errorf("expected scroll offset 0 after home, got %d", m.ScrollOffset)
	}
}

func TestModelStatusExpiry(t *testing.T) {
	m := NewModel(false)
	m.SetStatus("Done", false, 50*time.Millisecond)

	msg, isErr := m.ActiveStatus()
	if msg != "Done" || isErr {
		t.Errorf("expected active status 'Done', false; got %q, %v", msg, isErr)
	}

	time.Sleep(60 * time.Millisecond)
	msg, _ = m.ActiveStatus()
	if msg != "" {
		t.Errorf("expected expired status, got %q", msg)
	}
}
