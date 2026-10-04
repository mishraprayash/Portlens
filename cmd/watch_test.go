package cmd

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
)

func TestDiffWatch(t *testing.T) {
	prev := watchSnap{
		"Port 3000": "up:100:node",
		"Port 4000": "down",
	}
	cur := watchSnap{
		"Port 3000": "up:100:node",
		"Port 4000": "up:200:python",
		"Port 5000": "up:300:go",
	}
	got := diffWatch(prev, cur)
	want := []watchChange{
		{kind: "up", target: "Port 4000", detail: "up:200:python"},
		{kind: "up", target: "Port 5000", detail: "up:300:go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}

func TestDiffWatchDown(t *testing.T) {
	prev := watchSnap{"Port 3000": "up:1:node"}
	got := diffWatch(prev, watchSnap{"Port 3000": "down"})
	want := []watchChange{{kind: "down", target: "Port 3000"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}

func TestDiffWatchProcessChange(t *testing.T) {
	prev := watchSnap{"Port 3000": "up:1:node"}
	got := diffWatch(prev, watchSnap{"Port 3000": "up:2:node"})
	want := []watchChange{{kind: "changed", target: "Port 3000", detail: "up:2:node"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}

func TestNotifyChangesUsesInjectedNotifier(t *testing.T) {
	type call struct{ title, msg string }
	var calls []call
	plat := &platform.Platform{Notify: func(_ context.Context, title, msg string) error {
		calls = append(calls, call{title, msg})
		return nil
	}}

	notifyChanges(plat, context.Background(), []watchChange{
		{kind: "up", target: "3000", detail: "HTTP on localhost"},
		{kind: "down", target: "8080"},
	})

	if len(calls) != 2 {
		t.Fatalf("notifications = %d, want 2", len(calls))
	}
	if calls[0].title != "PortLens: up" || !strings.Contains(calls[0].msg, "now listening") {
		t.Errorf("up notification = %+v, want PortLens: up / now listening", calls[0])
	}
	if calls[1].title != "PortLens: down" || !strings.Contains(calls[1].msg, "no longer listening") {
		t.Errorf("down notification = %+v, want PortLens: down / no longer listening", calls[1])
	}
}

func TestWatchHelpersAndClearScreen(t *testing.T) {
	if got := watchTargetLabel(&options{}); got != "All listening ports" {
		t.Errorf("watchTargetLabel(no ports) = %q", got)
	}
	if got := watchTargetLabel(&options{ports: []int32{3000, 3001}}); got != "Ports 3000-3001" {
		t.Errorf("watchTargetLabel(ports) = %q", got)
	}
	if got := watchPortKey(8080); got != "Port 8080" {
		t.Errorf("watchPortKey = %q", got)
	}
	if got := watchListKey(model.PortEntry{Port: 80, Protocol: model.ProtocolTCP}); got != "Port 80 (tcp)" {
		t.Errorf("watchListKey = %q", got)
	}
	var out bytes.Buffer
	clearScreen(&out)
	if !strings.Contains(out.String(), "\x1b[2J\x1b[H") {
		t.Errorf("clearScreen output = %q, want clear escape", out.String())
	}
}

func TestRenderWatchTickPortDown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts := &options{ports: []int32{1}, noColor: true}
	s, err := renderWatchTick(context.Background(), &stdout, &stderr, opts)
	if err != nil {
		t.Fatalf("renderWatchTick = %v, want nil", err)
	}
	if s["Port 1"] != "down" {
		t.Errorf("snapshot = %q, want down", s["Port 1"])
	}
}

func TestRunPortsSinglePortLoop(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts := &options{ports: []int32{1}}
	if got := runPorts(context.Background(), &stdout, &stderr, strings.NewReader(""), opts); got != exitcode.PortNotFound {
		t.Errorf("runPorts(port 1) = %d, want %d", got, exitcode.PortNotFound)
	}
}
