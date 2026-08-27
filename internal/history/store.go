// Package history provides local, private port-observation history backed by an
// embedded SQLite database. History is stored on the user's machine only and is
// never transmitted anywhere.
package history

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/portlens/portlens/internal/model"
)

// Store persists and queries port observation history.
type Store interface {
	Record(ctx context.Context, entry model.HistoryEntry) error
	Query(ctx context.Context, port int32, limit int) ([]model.HistoryEntry, error)
	Close() error
}

// SQLiteStore is the default Store implementation.
type SQLiteStore struct {
	mu   sync.Mutex
	db   *sql.DB
	path string
}

// New opens (or creates) the history database at the default location.
func New() (*SQLiteStore, error) {
	return NewAt(DefaultPath())
}

// NewAt opens (or creates) the history database at an explicit path. It is
// useful for tests.
func NewAt(path string) (*SQLiteStore, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// A single connection avoids SQLite locking contention from concurrent
	// access to the same file.
	db.SetMaxOpenConns(1)
	s := &SQLiteStore{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS port_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    port        INTEGER NOT NULL,
    observed_at TEXT    NOT NULL,
    pid         INTEGER,
    process     TEXT,
    project     TEXT,
    command     TEXT,
    address     TEXT,
    status      TEXT
);
CREATE INDEX IF NOT EXISTS idx_port_history_port
    ON port_history (port, observed_at DESC);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Record(ctx context.Context, entry model.HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	const q = `INSERT INTO port_history
        (port, observed_at, pid, process, project, command, address, status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		entry.Port,
		entry.ObservedAt.Format(time.RFC3339Nano),
		nullableInt64(entry.PID),
		nullableString(entry.Process),
		nullableString(entry.Project),
		nullableString(entry.Command),
		nullableString(entry.Address),
		entry.Status,
	)
	return err
}

func (s *SQLiteStore) Query(ctx context.Context, port int32, limit int) ([]model.HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	const q = `SELECT port, observed_at, pid, process, project, command, address, status
        FROM port_history WHERE port = ? ORDER BY observed_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, port, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.HistoryEntry
	for rows.Next() {
		var e model.HistoryEntry
		var observed string
		var pid, project, process, command, address sql.NullString
		var status string
		if err := rows.Scan(&e.Port, &observed, &pid, &process, &project, &command, &address, &status); err != nil {
			return nil, err
		}
		e.ObservedAt, _ = time.Parse(time.RFC3339Nano, observed)
		e.PID = parseInt(pid)
		e.Process = process.String
		e.Project = project.String
		e.Command = command.String
		e.Address = address.String
		e.Status = status
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// DefaultPath returns the location of the history database for the current OS.
func DefaultPath() string {
	return filepath.Join(DataDir(), "history.db")
}

// DataDir returns the platform-appropriate data directory for PortLens,
// respecting XDG_DATA_HOME on Linux.
func DataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "portlens")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "portlens")
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "portlens")
		}
		return filepath.Join(home, ".portlens")
	default:
		return filepath.Join(home, ".local", "share", "portlens")
	}
}

func nullableInt64(v int32) any {
	if v == 0 {
		return nil
	}
	return int64(v)
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func parseInt(s sql.NullString) int32 {
	if !s.Valid {
		return 0
	}
	n, err := strconv.ParseInt(s.String, 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}
