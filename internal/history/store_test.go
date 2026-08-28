package history

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portlens/portlens/internal/model"
)

func TestRecordAndQuery(t *testing.T) {
	store, err := NewAt(filepath.Join(t.TempDir(), "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()
	if err := store.Record(ctx, model.HistoryEntry{
		Port:       3000,
		ObservedAt: now,
		PID:        1234,
		Process:    "node",
		Project:    "orbit-backend",
		Command:    "pnpm dev",
		Address:    "127.0.0.1",
		Status:     "seen",
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.Query(ctx, 3000, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.PID != 1234 || e.Process != "node" || e.Project != "orbit-backend" || e.Command != "pnpm dev" {
		t.Errorf("entry = %+v", e)
	}
	if !e.ObservedAt.Equal(now) {
		t.Errorf("observed_at = %v, want %v", e.ObservedAt, now)
	}
}

func TestQueryEmptyPort(t *testing.T) {
	store, err := NewAt(filepath.Join(t.TempDir(), "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	entries, err := store.Query(context.Background(), 9999, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestRecordMultipleOrdered(t *testing.T) {
	store, err := NewAt(filepath.Join(t.TempDir(), "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	for i, ts := range []time.Time{
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-1 * time.Hour),
		time.Now(),
	} {
		if err := store.Record(ctx, model.HistoryEntry{
			Port: 3000, ObservedAt: ts, PID: int32(100 + i), Status: "seen",
		}); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := store.Query(ctx, 3000, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	// Most recent first.
	if entries[0].PID != 102 || entries[2].PID != 100 {
		t.Errorf("entries not ordered newest-first: %+v", entries)
	}
}

func TestQuerySkipsTornLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store, err := NewAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Record(ctx, model.HistoryEntry{Port: 3000, PID: 1, Status: "seen"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(ctx, model.HistoryEntry{Port: 3000, PID: 2, Status: "seen"}); err != nil {
		t.Fatal(err)
	}
	// Append a torn record as the final line, as a crash mid-write would leave.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"port": 3000, "pid": 99`); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	entries, err := store.Query(ctx, 3000, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (torn line skipped)", len(entries))
	}
	if entries[0].PID != 2 || entries[1].PID != 1 {
		t.Errorf("entries not newest-first: %+v", entries)
	}
}

func TestHistoryFilePerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store, err := NewAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("history file perms = %o, want owner-only (0600)", perm)
	}
}
