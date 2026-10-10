package platform

import (
	"context"
	"sort"
	"sync"

	"github.com/portlens/portlens/internal/model"
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

// processTableTreeProvider builds process hierarchies from a single snapshot of
// the OS process table, so hierarchy operations are in-memory lookups instead
// of repeated system scans.
type processTableTreeProvider struct {
	once sync.Once
	rows []processRow
	byID map[int32]processRow
	kids map[int32][]int32
	err  error
}

func newProcessTreeProvider() ProcessTreeProvider {
	return &processTableTreeProvider{}
}

func (p *processTableTreeProvider) load() {
	p.once.Do(func() {
		p.byID = map[int32]processRow{}
		p.kids = map[int32][]int32{}
		rows, err := loadProcessTable()
		if err != nil {
			p.err = err
			return
		}
		p.rows = rows
		for _, r := range rows {
			p.byID[r.pid] = r
			if r.ppid > 0 {
				p.kids[r.ppid] = append(p.kids[r.ppid], r.pid)
			}
		}
		for _, kids := range p.kids {
			sort.Slice(kids, func(i, j int) bool { return kids[i] < kids[j] })
		}
	})
}

// Ancestors returns the chain from the PID up to the root, oldest first.
func (p *processTableTreeProvider) Ancestors(_ context.Context, pid int32) ([]*model.ProcessInfo, error) {
	p.load()
	if p.err != nil {
		return nil, p.err
	}
	var chain []*model.ProcessInfo
	seen := map[int32]bool{}
	cur := pid
	for depth := 0; depth < maxTreeDepth; depth++ {
		if seen[cur] {
			break
		}
		seen[cur] = true
		row, ok := p.byID[cur]
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
	p.load()
	if p.err != nil {
		return nil, p.err
	}
	kids := p.kids[pid]
	out := make([]*model.ProcessInfo, 0, len(kids))
	for _, k := range kids {
		r := p.byID[k]
		out = append(out, &model.ProcessInfo{PID: r.pid, PPID: r.ppid, Name: r.name})
	}
	return out, nil
}

// Descendants returns the full descendant tree from the same snapshot.
func (p *processTableTreeProvider) Descendants(_ context.Context, pid int32) (*model.ProcessTree, error) {
	p.load()
	if p.err != nil {
		return nil, p.err
	}
	row, ok := p.byID[pid]
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
		for _, k := range p.kids[current] {
			kr := p.byID[k]
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
