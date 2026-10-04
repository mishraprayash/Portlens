package platform

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

// processRow is the minimal identity needed for hierarchy operations. Building
// the whole table once and answering Ancestors/Children/Descendants in memory
// is far cheaper than the per-call process enumeration it replaces.
type processRow struct {
	pid  int32
	ppid int32
	name string
}

// maxTreeDepth guards against cycles or corrupted parent links in the table.
const maxTreeDepth = 64

// processTable is an immutable snapshot of the OS process table: hierarchy
// operations are in-memory lookups instead of repeated system scans.
type processTable struct {
	byID map[int32]processRow
	kids map[int32][]int32
}

// processTableTreeProvider serves hierarchy queries from a short-lived
// snapshot. The snapshot expires after procTableTTL so long-lived processes
// (TUI, watch) see children spawned after startup and never follow recycled
// PIDs; failed builds are never cached.
type processTableTreeProvider struct {
	mu      sync.Mutex
	builtAt time.Time
	built   bool
	tbl     *processTable
	err     error
	now     func() time.Time
	ttl     time.Duration
}

// procTableTTL bounds how stale the cached process hierarchy may be.
const procTableTTL = time.Second

func newProcessTreeProvider() ProcessTreeProvider {
	return &processTableTreeProvider{now: time.Now, ttl: procTableTTL}
}

// fresh reports whether the cached snapshot may still be used at time now.
// Failed builds are never fresh, so a transient enumeration error does not
// stick for the process lifetime.
func (p *processTableTreeProvider) fresh(now time.Time) bool {
	return p.built && p.err == nil && now.Sub(p.builtAt) < p.ttl
}

// table returns the current snapshot, rebuilding it when expired. The
// returned table is immutable; a rebuild swaps in a new one, so callers that
// hold it are unaffected by concurrent refreshes.
func (p *processTableTreeProvider) table() (*processTable, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fresh(p.now()) {
		return p.tbl, p.err
	}
	p.err = nil
	p.built = false
	rows, err := loadProcessTable()
	if err != nil {
		p.err = err
		return nil, err
	}
	tbl := &processTable{
		byID: map[int32]processRow{},
		kids: map[int32][]int32{},
	}
	for _, r := range rows {
		tbl.byID[r.pid] = r
		if r.ppid > 0 {
			tbl.kids[r.ppid] = append(tbl.kids[r.ppid], r.pid)
		}
	}
	for _, kids := range tbl.kids {
		sort.Slice(kids, func(i, j int) bool { return kids[i] < kids[j] })
	}
	p.tbl = tbl
	p.builtAt = p.now()
	p.built = true
	return tbl, nil
}

// Ancestors returns the chain from the PID up to the root, oldest first.
func (p *processTableTreeProvider) Ancestors(_ context.Context, pid int32) ([]*model.ProcessInfo, error) {
	tbl, err := p.table()
	if err != nil {
		return nil, err
	}
	var chain []*model.ProcessInfo
	seen := map[int32]bool{}
	cur := pid
	for depth := 0; depth < maxTreeDepth; depth++ {
		if seen[cur] {
			break
		}
		seen[cur] = true
		row, ok := tbl.byID[cur]
		if !ok {
			break
		}
		chain = append(chain, &model.ProcessInfo{PID: row.pid, PPID: row.ppid, Name: row.name})
		if row.ppid <= 0 || row.ppid == cur {
			break
		}
		cur = row.ppid
	}
	reverseInfos(chain)
	return chain, nil
}

// Children returns the direct children of a PID from the same snapshot.
func (p *processTableTreeProvider) Children(_ context.Context, pid int32) ([]*model.ProcessInfo, error) {
	tbl, err := p.table()
	if err != nil {
		return nil, err
	}
	kids := tbl.kids[pid]
	out := make([]*model.ProcessInfo, 0, len(kids))
	for _, k := range kids {
		r := tbl.byID[k]
		out = append(out, &model.ProcessInfo{PID: r.pid, PPID: r.ppid, Name: r.name})
	}
	return out, nil
}

// Descendants returns the full descendant tree from the same snapshot.
func (p *processTableTreeProvider) Descendants(_ context.Context, pid int32) (*model.ProcessTree, error) {
	tbl, err := p.table()
	if err != nil {
		return nil, err
	}
	row, ok := tbl.byID[pid]
	if !ok {
		return nil, ErrProcessNotFound
	}
	root := &model.ProcessTree{Process: model.ProcessInfo{PID: row.pid, PPID: row.ppid, Name: row.name}}
	var build func(node *model.ProcessTree, current int32, depth int, seen map[int32]bool)
	build = func(node *model.ProcessTree, current int32, depth int, seen map[int32]bool) {
		if depth >= maxTreeDepth || seen[current] {
			return
		}
		seen[current] = true
		for _, k := range tbl.kids[current] {
			kr := tbl.byID[k]
			child := &model.ProcessTree{Process: model.ProcessInfo{PID: kr.pid, PPID: kr.ppid, Name: kr.name}}
			node.Children = append(node.Children, child)
			build(child, k, depth+1, seen)
		}
	}
	build(root, pid, 0, map[int32]bool{})
	return root, nil
}

func reverseInfos(s []*model.ProcessInfo) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
