package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/actions"
	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/inspector"
	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
	"github.com/mishraprayash/Portlens/internal/render"
)

func TestRunAction(t *testing.T) {
	if got := runAction(io.Discard, "kill", func() error { return nil }, nil); got != exitcode.Success {
		t.Errorf("runAction(success) = %d, want 0", got)
	}

	var out bytes.Buffer
	got := runAction(&out, "kill", func() error { return errors.New("boom") }, nil)
	if got != exitcode.GeneralError {
		t.Errorf("runAction(error) = %d, want %d", got, exitcode.GeneralError)
	}
	if !strings.Contains(out.String(), "kill failed: boom") {
		t.Errorf("output = %q, want kill failed: boom", out.String())
	}

	out.Reset()
	specialErr := errors.New("special")
	got = runAction(&out, "restart", func() error { return specialErr }, func(err error) (int, string, bool) {
		if errors.Is(err, specialErr) {
			return exitcode.ProcessActionFailed, "special message\n", true
		}
		return 0, "", false
	})
	if got != exitcode.ProcessActionFailed {
		t.Errorf("runAction(special) = %d, want %d", got, exitcode.ProcessActionFailed)
	}
	if !strings.Contains(out.String(), "special message") {
		t.Errorf("output = %q, want special message", out.String())
	}
}

func TestRunKillWithoutProcess(t *testing.T) {
	var out bytes.Buffer
	mgr := &actions.Manager{Platform: &platform.Platform{}, Out: &out, Wait: time.Second}
	code := runKill(context.Background(), mgr, &model.Report{Port: 9}, &options{})
	if code != exitcode.GeneralError {
		t.Errorf("runKill(no process) = %d, want %d", code, exitcode.GeneralError)
	}
	if !strings.Contains(out.String(), "kill failed: no owning process to terminate") {
		t.Errorf("output = %q, want kill failure message", out.String())
	}
}

func TestRunRestartUnavailable(t *testing.T) {
	var out bytes.Buffer
	mgr := &actions.Manager{Platform: &platform.Platform{}, Out: &out, Wait: time.Second}
	code := runRestart(context.Background(), mgr, &model.Report{Port: 9})
	if code != exitcode.ProcessActionFailed {
		t.Errorf("runRestart(unavailable) = %d, want %d", code, exitcode.ProcessActionFailed)
	}
	if !strings.Contains(out.String(), "Automatic restart is unavailable.") {
		t.Errorf("output = %q, want unavailability message", out.String())
	}
}

func TestRunOpenUsesInjectedOpener(t *testing.T) {
	var got string
	mgr := &actions.Manager{
		Platform: &platform.Platform{OpenURL: func(_ context.Context, u string) error {
			got = u
			return nil
		}},
		Out:  io.Discard,
		Wait: time.Second,
	}
	code := runOpen(context.Background(), mgr, &model.Report{Port: 3000, Address: "127.0.0.1"})
	if code != exitcode.Success {
		t.Errorf("runOpen = %d, want 0", code)
	}
	if got != "http://localhost:3000" {
		t.Errorf("opener received %q, want http://localhost:3000", got)
	}
}

func TestInspectDepth(t *testing.T) {
	cases := []struct {
		name string
		opts *options
		want inspector.Depth
	}{
		{"default", &options{}, inspector.DepthFast},
		{"verbose", &options{verbose: true}, inspector.DepthFull},
		{"tree", &options{tree: true}, inspector.DepthFull},
		{"connections", &options{connections: true}, inspector.DepthFull},
		{"restart", &options{restart: true}, inspector.DepthFull},
		{"json", &options{jsonOut: true}, inspector.DepthFull},
	}
	for _, c := range cases {
		if got := inspectDepth(c.opts); got != c.want {
			t.Errorf("%s: inspectDepth = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRenderReportModes(t *testing.T) {
	report := &model.Report{Port: 3000, Status: "listening", Protocol: model.ProtocolTCP}
	var compact, verbose bytes.Buffer
	r1 := render.New(&compact, false)
	renderReport(r1, report, &options{})
	r2 := render.New(&verbose, false)
	renderReport(r2, report, &options{verbose: true})
	if compact.Len() == 0 {
		t.Error("compact render is empty")
	}
	if verbose.Len() == 0 {
		t.Error("verbose render is empty")
	}
	if compact.String() == verbose.String() {
		t.Error("verbose render should differ from compact summary")
	}
}

func TestRunPortNotFound(t *testing.T) {
	// Port 1 is privileged and never listening under the test runner.
	var stdout, stderr bytes.Buffer
	opts := &options{}
	code := runPort(context.Background(), &stdout, &stderr, strings.NewReader(""), opts, 1)
	if code != exitcode.PortNotFound {
		t.Errorf("runPort(port 1) = %d, want %d (stderr: %s)", code, exitcode.PortNotFound, stderr.String())
	}
}
