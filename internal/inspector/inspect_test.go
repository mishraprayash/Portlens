package inspector

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
)

// fakeHost is a configurable in-memory host implementing every provider the
// inspector consumes.
type fakeHost struct {
	listeners []model.Listener
	procs     map[int32]*model.ProcessInfo
	conns     []model.Connection
	ancestors map[int32][]*model.ProcessInfo
	portsErr  error
}

func (f *fakeHost) Listeners(context.Context) ([]model.Listener, error) {
	return f.listeners, f.portsErr
}
func (f *fakeHost) ResolvePort(_ context.Context, port uint16, _ model.Protocol) ([]model.Listener, error) {
	if f.portsErr != nil {
		return nil, f.portsErr
	}
	var out []model.Listener
	for _, l := range f.listeners {
		if l.Port == port {
			out = append(out, l)
		}
	}
	return out, nil
}
func (f *fakeHost) Info(_ context.Context, pid int32) (*model.ProcessInfo, error) {
	if p, ok := f.procs[pid]; ok {
		c := *p
		return &c, nil
	}
	return nil, errors.New("process not found")
}
func (f *fakeHost) InfoBasic(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	return f.Info(ctx, pid)
}
func (f *fakeHost) Exists(_ context.Context, pid int32) bool { _, ok := f.procs[pid]; return ok }
func (f *fakeHost) Connections(context.Context, int32) ([]model.Connection, error) {
	return f.conns, nil
}
func (f *fakeHost) Ancestors(_ context.Context, pid int32) ([]*model.ProcessInfo, error) {
	return f.ancestors[pid], nil
}
func (f *fakeHost) Children(context.Context, int32) ([]*model.ProcessInfo, error) { return nil, nil }
func (f *fakeHost) Descendants(_ context.Context, pid int32) (*model.ProcessTree, error) {
	return &model.ProcessTree{Process: *f.procs[pid]}, nil
}

func (f *fakeHost) inspector() *Inspector {
	return New(&platform.Platform{Ports: f, Processes: f, Network: f, Tree: f})
}

func newHost(t *testing.T) *fakeHost {
	t.Helper()
	cwd := t.TempDir()
	return &fakeHost{
		listeners: []model.Listener{
			{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 3000, State: "LISTEN", PID: 10},
			{Protocol: model.ProtocolTCP, Address: "0.0.0.0", Port: 5432, State: "LISTEN", PID: 20},
			{Protocol: model.ProtocolTCP, Address: "127.0.0.1", Port: 9000, State: "LISTEN"}, // unknown owner
		},
		procs: map[int32]*model.ProcessInfo{
			10: {PID: 10, PPID: 5, Name: "node", Command: "node server.js", Cmdline: []string{"node", "server.js"}, CWD: cwd, User: "dev", StartTime: time.Now().Add(-time.Hour)},
			20: {PID: 20, PPID: 1, Name: "postgres", Command: "postgres -D /data", CWD: cwd},
		},
		conns: []model.Connection{
			{State: "ESTABLISHED", RemoteAddr: "10.0.0.5"},
			{State: "ESTABLISHED", RemoteAddr: "127.0.0.1"},
			{State: "TIME_WAIT", RemoteAddr: "::1"},
			{RemoteAddr: ""},
		},
		ancestors: map[int32][]*model.ProcessInfo{
			10: {{PID: 1, Name: "launchd"}, {PID: 5, Name: "zsh"}, {PID: 10, Name: "node"}},
		},
	}
}

func TestInspectFullBuildsReport(t *testing.T) {
	rep, err := newHost(t).inspector().Inspect(context.Background(), 3000, model.ProtocolTCP)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != "listening" || rep.Process == nil || rep.Process.PID != 10 || !rep.Process.IsTarget {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Network == nil || rep.Network.Summary.Total != 4 {
		t.Fatalf("network = %+v", rep.Network)
	}
	if len(rep.Ancestors) != 3 {
		t.Errorf("ancestors = %d", len(rep.Ancestors))
	}
	if rep.Interpretation == "" || len(rep.Facts) == 0 {
		t.Errorf("deep inspection should interpret: %q %v", rep.Interpretation, rep.Facts)
	}
	joined := strings.Join(rep.Facts, "\n")
	for _, want := range []string{"127.0.0.1:3000", "Full command: node server.js", `user "dev"`, "Parent process: zsh"} {
		if !strings.Contains(joined, want) {
			t.Errorf("facts missing %q:\n%s", want, joined)
		}
	}
}

func TestInspectFastSkipsDeepWork(t *testing.T) {
	rep, err := newHost(t).inspector().InspectDepth(context.Background(), 3000, model.ProtocolTCP, DepthFast)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Network != nil || rep.Ancestors != nil || rep.Interpretation != "" || len(rep.Facts) != 0 {
		t.Errorf("fast path did deep work: %+v", rep)
	}
	if rep.Exposure == nil {
		t.Error("exposure is part of the fast path")
	}
}

func TestInspectNotListening(t *testing.T) {
	rep, err := newHost(t).inspector().Inspect(context.Background(), 1234, model.ProtocolTCP)
	if !errors.Is(err, ErrPortNotFound) || rep.Status != "not_listening" {
		t.Fatalf("err = %v, report = %+v", err, rep)
	}
}

func TestInspectPropagatesResolverError(t *testing.T) {
	h := newHost(t)
	h.portsErr = errors.New("lsof exploded")
	if _, err := h.inspector().Inspect(context.Background(), 3000, model.ProtocolTCP); err == nil {
		t.Fatal("expected error")
	}
}

func TestInspectUnknownOwner(t *testing.T) {
	rep, err := newHost(t).inspector().Inspect(context.Background(), 9000, model.ProtocolTCP)
	if err != nil || rep.Process != nil || rep.Status != "listening" {
		t.Fatalf("err = %v, report = %+v", err, rep)
	}
	if len(rep.Inferences) == 0 || !strings.Contains(rep.Inferences[0], "Owner could not be determined") {
		t.Errorf("inferences = %v", rep.Inferences)
	}
}

func TestInspectOwnerExitedBeforeLookup(t *testing.T) {
	h := newHost(t)
	delete(h.procs, 10)
	rep, err := h.inspector().Inspect(context.Background(), 3000, model.ProtocolTCP)
	if err != nil || rep.Process != nil {
		t.Fatalf("err = %v, report = %+v", err, rep)
	}
	if len(rep.Facts) == 0 || !strings.Contains(rep.Facts[0], "has since exited") {
		t.Errorf("facts = %v", rep.Facts)
	}
}

func TestListEntries(t *testing.T) {
	entries, err := newHost(t).inspector().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].Port != 3000 || entries[1].Service != "PostgreSQL" {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Process != "node" || entries[2].Process != "" {
		t.Errorf("process attribution wrong: %+v", entries)
	}
}

func TestListPropagatesError(t *testing.T) {
	h := newHost(t)
	h.portsErr = errors.New("nope")
	if _, err := h.inspector().List(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchByName(t *testing.T) {
	insp := newHost(t).inspector()
	got, err := insp.SearchByName(context.Background(), "POSTGRES")
	if err != nil || len(got) != 1 || got[0].Port != 5432 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	got, _ = insp.SearchByName(context.Background(), "/^node\\b/")
	if len(got) != 1 || got[0].Port != 3000 {
		t.Fatalf("regex got %+v", got)
	}
	if _, err := insp.SearchByName(context.Background(), "/(/"); err == nil {
		t.Error("invalid regex should error")
	}
	if _, err := insp.SearchByName(context.Background(), "  "); err == nil {
		t.Error("empty query should error")
	}
}

func TestSearchByPIDIncludesDescendants(t *testing.T) {
	h := newHost(t)
	insp := h.inspector()
	// zsh (pid 5) is an ancestor of node (pid 10).
	got, err := insp.SearchByPID(context.Background(), 5)
	if err != nil || len(got) != 1 || got[0].Port != 3000 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	got, _ = insp.SearchByPID(context.Background(), 20)
	if len(got) != 1 || got[0].Port != 5432 {
		t.Fatalf("direct owner got %+v", got)
	}
	got, _ = insp.SearchByPID(context.Background(), 999)
	if len(got) != 0 {
		t.Fatalf("unrelated pid matched %+v", got)
	}
	h.portsErr = errors.New("nope")
	if _, err := insp.SearchByPID(context.Background(), 5); err == nil {
		t.Error("expected resolver error")
	}
}

func TestSummarize(t *testing.T) {
	s := summarize([]model.Connection{
		{State: "ESTABLISHED", RemoteAddr: "8.8.8.8"},
		{State: "ESTABLISHED", RemoteAddr: "127.0.0.1"},
		{RemoteAddr: ""},
	})
	if s.Total != 3 || s.ByState["ESTABLISHED"] != 2 || s.ByState["UNKNOWN"] != 1 || s.RemoteCount != 1 || s.LocalOnly {
		t.Errorf("summary = %+v", s)
	}
	if !summarize(nil).LocalOnly {
		t.Error("no connections should be local-only")
	}
}

func TestIsLoopbackAndDisplayAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1": true, "127.5.5.5": true, "::1": true, "localhost": true,
		"0.0.0.0": false, "10.0.0.1": false, "": false,
	} {
		if got := isLoopback(addr); got != want {
			t.Errorf("isLoopback(%q) = %v", addr, got)
		}
	}
	if displayAddr("") != "0.0.0.0" || displayAddr("1.2.3.4") != "1.2.3.4" {
		t.Error("displayAddr wrong")
	}
}

func TestBuildInterpretation(t *testing.T) {
	cases := []struct{ rt, fw, want string }{
		{"", "", "Unknown process"},
		{"node", "", "Node.js process"},
		{"node", "nestjs", "NestJS process (Node.js)"},
	}
	for _, c := range cases {
		if got := buildInterpretation(c.rt, c.fw); got != c.want {
			t.Errorf("buildInterpretation(%q,%q) = %q, want %q", c.rt, c.fw, got, c.want)
		}
	}
	if parentName(&model.Report{}) != "" {
		t.Error("parentName with no ancestors should be empty")
	}
}
