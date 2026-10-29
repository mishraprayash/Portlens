package detect

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

// projectMemoTTL bounds how long a detection result — including a negative
// one — may be reused for the same working directory: long enough that a scan
// of many ports in one process tree pays a single directory walk, short
// enough that a git checkout or a newly created package.json shows up within
// a few seconds.
const projectMemoTTL = 5 * time.Second

// projectMemoMaxEntries caps the cache before expired entries are pruned,
// bounding memory for long-lived watch/TUI processes that see many working
// directories over time.
const projectMemoMaxEntries = 128

type projectMemoEntry struct {
	info *model.ProjectInfo
	at   time.Time
}

// memoProjectDetector caches Detect results per working directory.
type memoProjectDetector struct {
	inner   ProjectDetector
	ttl     time.Duration
	now     func() time.Time
	mu      sync.Mutex
	entries map[string]projectMemoEntry
}

// Memoize wraps a ProjectDetector with a per-CWD TTL cache. Negative results
// (nil) are cached too — most processes are not in a project — and failed or
// expired entries are recomputed on the next call.
func Memoize(d ProjectDetector) ProjectDetector {
	return &memoProjectDetector{
		inner:   d,
		ttl:     projectMemoTTL,
		now:     time.Now,
		entries: map[string]projectMemoEntry{},
	}
}

func (m *memoProjectDetector) Detect(ctx context.Context, cwd string) *model.ProjectInfo {
	key := filepath.Clean(cwd)

	m.mu.Lock()
	if e, ok := m.entries[key]; ok && m.now().Sub(e.at) < m.ttl {
		info := e.info
		m.mu.Unlock()
		return info
	}
	m.mu.Unlock()

	info := m.inner.Detect(ctx, cwd)

	m.mu.Lock()
	if len(m.entries) >= projectMemoMaxEntries {
		now := m.now()
		for k, e := range m.entries {
			if now.Sub(e.at) >= m.ttl {
				delete(m.entries, k)
			}
		}
	}
	m.entries[key] = projectMemoEntry{info: info, at: m.now()}
	m.mu.Unlock()
	return info
}
