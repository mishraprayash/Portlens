// Package history provides local, private port-observation history backed by a
// per-line JSON log file. History is stored on the user's machine only and is
// never transmitted anywhere. The log is append-only: each Record is one
// atomic O_APPEND write, and Query scans the file defensively, skipping any
// malformed line. If a crash tears a write mid-line (leaving no trailing
// newline), the torn record and the record appended after it share one physical
// line and both are skipped; this is accepted for best-effort local history.
package history

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/portlens/portlens/internal/config"
	"github.com/portlens/portlens/internal/model"
)

// Store persists and queries port observation history.
type Store interface {
	Record(ctx context.Context, entry model.HistoryEntry) error
	Query(ctx context.Context, port int32, limit int) ([]model.HistoryEntry, error)
	Close() error
}

// JSONLStore appends JSON records to a private log file.
type JSONLStore struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// New opens (or creates) the history log at the default location.
func New() (*JSONLStore, error) {
	return NewAt(DefaultPath())
}

// NewAt opens (or creates) the history log at an explicit path. The file is
// created with owner-only permissions (0600) because it records which
// processes and projects ran on this machine.
func NewAt(path string) (*JSONLStore, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &JSONLStore{path: path, f: f}, nil
}

// Record appends one observation as a JSON line. A single O_APPEND write keeps
// concurrent PortLens processes from interleaving records.
func (s *JSONLStore) Record(_ context.Context, entry model.HistoryEntry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.f.Write(line)
	return err
}

// Query returns up to limit most recent observations for a port, newest first.
// Malformed lines (e.g. a record torn by a crash) are skipped rather than
// failing the whole read.
func (s *JSONLStore) Query(_ context.Context, port int32, limit int) ([]model.HistoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.f.Sync(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var out []model.HistoryEntry
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e model.HistoryEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // torn or corrupt record; skip
		}
		if e.Port == port {
			out = append(out, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	// The file is chronological (oldest first); return the most recent `limit`.
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	reverse(out)
	return out, nil
}

func (s *JSONLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}

func reverse(s []model.HistoryEntry) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// DefaultPath returns the location of the history log for the current OS.
func DefaultPath() string {
	return filepath.Join(DataDir(), "history.jsonl")
}

// DataDir returns the platform-appropriate data directory for PortLens. It is
// a thin wrapper around config.Dir so every component agrees on one location.
func DataDir() string {
	return config.Dir()
}
