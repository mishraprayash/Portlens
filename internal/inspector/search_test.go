package inspector

import (
	"context"
	"testing"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/model"
)

func TestCompileProcessMatcher(t *testing.T) {
	if _, err := compileProcessMatcher(""); err == nil {
		t.Error("expected error for empty query")
	}
	if _, err := compileProcessMatcher("   "); err == nil {
		t.Error("expected error for whitespace-only query")
	}
	if _, err := compileProcessMatcher("/[/"); err == nil {
		t.Error("expected error for invalid regex")
	}
	if _, err := compileProcessMatcher("/node/"); err != nil {
		t.Error("expected valid regex to compile")
	}
}

func TestProcessMatcherMatches(t *testing.T) {
	base := &model.ProcessInfo{
		Name:    "node",
		Command: "pnpm dev",
		Exe:     "/usr/local/bin/node",
		Cmdline: []string{"/usr/local/bin/node", "/usr/bin/pnpm", "dev"},
	}
	cases := []struct {
		query string
		proc  *model.ProcessInfo
		want  bool
	}{
		{"node", base, true},
		{"NODE", base, true},
		{"pnpm dev", base, true},
		{"/usr/local/bin/node", base, true},
		{"python", base, false},
		{"/^node/", base, true},
		{"/dev$/", base, true},
		{"/python|node/", base, true},
		{"/^python/", base, false},
		{"node", nil, false},
	}
	for _, c := range cases {
		m, err := compileProcessMatcher(c.query)
		if err != nil {
			t.Fatalf("compileProcessMatcher(%q): %v", c.query, err)
		}
		if got := m.matches(c.proc); got != c.want {
			t.Errorf("matcher(%q).matches(%+v) = %v, want %v", c.query, c.proc, got, c.want)
		}
	}
}

func TestBuildEntries(t *testing.T) {
	insp := &Inspector{Projects: detect.NewProjectDetector()}
	listeners := []model.Listener{
		{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 3000, State: "LISTEN", PID: 11, Process: "node"},
		{Protocol: model.ProtocolUDP, Address: "0.0.0.0", Port: 3000, State: "BOUND", PID: 11, Process: "node"},
		{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 4000, State: "LISTEN", PID: 12, Process: "python"},
		{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 5000, State: "LISTEN", PID: 0, Process: "root-daemon"},
	}
	infos := map[int32]*model.ProcessInfo{
		11: {PID: 11, Name: "node"},
		12: {PID: 12, Name: "python"},
	}

	entries := insp.buildEntries(context.Background(), listeners, infos, nil)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Port != 3000 || entries[2].Port != 4000 || entries[3].Port != 5000 {
		t.Errorf("entries not sorted by port: %+v", entries)
	}
	if entries[3].Process != "root-daemon" {
		t.Errorf("listener without process info should fall back to l.Process: %+v", entries[3])
	}

	kept := insp.buildEntries(context.Background(), listeners, infos, func(p *model.ProcessInfo) bool {
		return p != nil && p.Name == "node"
	})
	if len(kept) != 2 || kept[0].Port != 3000 || kept[1].Port != 3000 {
		t.Errorf("keep filter: got %+v, want two port 3000 entries (tcp+udp)", kept)
	}
}

func TestBuildEntriesServiceAndOrigin(t *testing.T) {
	insp := &Inspector{Projects: detect.NewProjectDetector()}
	listeners := []model.Listener{
		{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 5432, State: "LISTEN", PID: 21, Process: "postgres"},
		{Protocol: model.ProtocolTCP, Address: "0.0.0.0", Port: 88, State: "LISTEN", PID: 22, Process: "kdc"},
		{Protocol: model.ProtocolUDP, Address: "0.0.0.0", Port: 88, State: "BOUND", PID: 22, Process: "kdc"},
	}
	infos := map[int32]*model.ProcessInfo{
		21: {PID: 21, Name: "postgres", Exe: "/opt/homebrew/opt/postgresql@16/bin/postgres"},
		22: {PID: 22, Name: "kdc", Exe: "/System/Library/PrivateFrameworks/Heimdal.framework/Helpers/kdc"},
	}

	entries := insp.buildEntries(context.Background(), listeners, infos, nil)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}
	byPort := map[int32]model.PortEntry{}
	for _, e := range entries {
		byPort[e.Port] = e
	}
	if got := byPort[5432]; got.Service != "PostgreSQL" || got.Origin != model.OriginUser {
		t.Errorf("postgres entry = %+v, want Service=PostgreSQL Origin=user", got)
	}
	if got := byPort[88]; got.Service != "Kerberos" || got.Origin != model.OriginSystem {
		t.Errorf("kdc entry = %+v, want Service=Kerberos Origin=system", got)
	}
}
