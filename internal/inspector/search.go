package inspector

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/model"
)

// SearchByName returns the listening ports owned by processes whose name,
// command line, or executable path matches the query. Matching is
// case-insensitive; a query wrapped in /.../ is treated as a regular
// expression.
func (i *Inspector) SearchByName(ctx context.Context, query string) ([]model.PortEntry, error) {
	matcher, err := compileProcessMatcher(query)
	if err != nil {
		return nil, err
	}
	listeners, err := i.Platform.Ports.Listeners(ctx)
	if err != nil {
		return nil, err
	}
	infos := i.processInfos(ctx, listeners)
	entries := i.buildEntries(ctx, listeners, infos, func(p *model.ProcessInfo) bool {
		return matcher.matches(p)
	})
	i.attachContainers(ctx, entries)
	return entries, nil
}

// SearchByPID returns the listening ports owned by the given PID or by any of
// its descendants, so supervisor processes (for example a Node or dev-server
// parent) match too.
func (i *Inspector) SearchByPID(ctx context.Context, pid int32) ([]model.PortEntry, error) {
	listeners, err := i.Platform.Ports.Listeners(ctx)
	if err != nil {
		return nil, err
	}
	infos := i.processInfos(ctx, listeners)
	ancestors := map[int32]bool{}
	contains := func(owner int32) bool {
		if owner == pid {
			return true
		}
		if hit, ok := ancestors[owner]; ok {
			return hit
		}
		hit := false
		if chain, err := i.Platform.Tree.Ancestors(ctx, owner); err == nil {
			for _, a := range chain {
				if a != nil && a.PID == pid {
					hit = true
					break
				}
			}
		}
		ancestors[owner] = hit
		return hit
	}
	entries := i.buildEntries(ctx, listeners, infos, func(p *model.ProcessInfo) bool {
		return p != nil && contains(p.PID)
	})
	i.attachContainers(ctx, entries)
	return entries, nil
}

// processInfos resolves process metadata for each unique listener PID.
func (i *Inspector) processInfos(ctx context.Context, listeners []model.Listener) map[int32]*model.ProcessInfo {
	out := map[int32]*model.ProcessInfo{}
	for _, l := range listeners {
		if l.PID <= 0 {
			continue
		}
		if _, ok := out[l.PID]; ok {
			continue
		}
		if p, err := i.Platform.Processes.Info(ctx, l.PID); err == nil {
			out[l.PID] = p
		}
	}
	return out
}

// buildEntries deduplicates listeners by family+port, enriches them into
// PortEntry rows (project, runtime), and keeps only those whose owning process
// satisfies keep. A nil keep keeps every listener, mirroring the List command.
func (i *Inspector) buildEntries(
	ctx context.Context,
	listeners []model.Listener,
	infos map[int32]*model.ProcessInfo,
	keep func(*model.ProcessInfo) bool,
) []model.PortEntry {
	seen := map[string]bool{}
	var out []model.PortEntry
	for _, l := range listeners {
		key := l.Key()
		if seen[key] {
			continue
		}
		seen[key] = true
		p := infos[l.PID]
		if keep != nil && !keep(p) {
			continue
		}
		entry := model.PortEntry{
			Port:     int32(l.Port),
			Protocol: l.Protocol,
			Address:  l.Address,
			Status:   l.State,
			PID:      l.PID,
			Service:  detect.LookupService(uint16(l.Port)),
		}
		if p != nil {
			entry.Process = p.Name
			entry.Origin = detect.ProcessOrigin(p)
			if proj := i.Projects.Detect(ctx, p.CWD); proj != nil {
				entry.Project = proj.Name
			}
			if rt := detect.DetectRuntime(p); rt != "" {
				entry.Runtime = rt
			}
		} else if l.Process != "" {
			entry.Process = l.Process
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

// processMatcher matches a process against a user query using case-insensitive
// substring semantics or, when the query is wrapped in /.../, a regular
// expression.
type processMatcher struct {
	re  *regexp.Regexp
	sub string
}

func compileProcessMatcher(query string) (*processMatcher, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty --name query")
	}
	if len(q) >= 2 && strings.HasPrefix(q, "/") && strings.HasSuffix(q, "/") {
		re, err := regexp.Compile("(?i)" + q[1:len(q)-1])
		if err != nil {
			return nil, fmt.Errorf("invalid --name regex %q: %w", q, err)
		}
		return &processMatcher{re: re}, nil
	}
	return &processMatcher{sub: strings.ToLower(q)}, nil
}

func (m *processMatcher) matches(p *model.ProcessInfo) bool {
	if p == nil {
		return false
	}
	var sb strings.Builder
	sb.WriteString(p.Name)
	sb.WriteByte(' ')
	sb.WriteString(p.Command)
	sb.WriteByte(' ')
	sb.WriteString(p.Exe)
	if len(p.Cmdline) > 0 {
		sb.WriteByte(' ')
		sb.WriteString(strings.Join(p.Cmdline, " "))
	}
	hay := strings.ToLower(sb.String())
	if m.re != nil {
		return m.re.MatchString(hay)
	}
	return strings.Contains(hay, m.sub)
}
