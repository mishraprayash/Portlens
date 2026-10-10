package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mishraprayash/Portlens/internal/actions"
	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/inspector"
	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
	"github.com/mishraprayash/Portlens/internal/render"
)

func TestRunInteractiveNonTerminalInput(t *testing.T) {
	var stdout bytes.Buffer
	insp := inspector.New(platform.New())
	mgr := &actions.Manager{Platform: &platform.Platform{}, Out: &stdout}
	r := render.New(&stdout, false)
	report := &model.Report{Port: 3000, Status: "listening", Protocol: model.ProtocolTCP}

	// stdin is not an *os.File, so the loop is skipped after rendering.
	code := runInteractive(context.Background(), insp, mgr, r, report, strings.NewReader("q"), &stdout, &options{})
	if code != exitcode.Success {
		t.Errorf("runInteractive = %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Error("expected the report to be rendered before returning")
	}
}

func TestDeepInteractiveReportFallsBack(t *testing.T) {
	insp := inspector.New(platform.New())
	orig := &model.Report{Port: 1, Status: "not_listening", Protocol: model.ProtocolTCP}
	got := deepInteractiveReport(context.Background(), insp, orig)
	if got != orig {
		t.Errorf("deepInteractiveReport on an inspect failure = %+v, want the original report", got)
	}
}

type fakeClipboard struct {
	text string
	err  error
}

func (f *fakeClipboard) Copy(_ context.Context, s string) error {
	f.text = s
	return f.err
}

func TestCopyText(t *testing.T) {
	clip := &fakeClipboard{}
	mgr := &actions.Manager{Platform: &platform.Platform{Clipboard: clip}}

	var out bytes.Buffer
	copyText(context.Background(), mgr, &out, "", "PID")
	if !strings.Contains(out.String(), "nothing to copy") {
		t.Errorf("empty copy output = %q, want nothing to copy", out.String())
	}

	out.Reset()
	copyText(context.Background(), mgr, &out, "42", "PID")
	if clip.text != "42" {
		t.Errorf("clipboard = %q, want 42", clip.text)
	}
	if !strings.Contains(out.String(), "copied PID to clipboard: 42") {
		t.Errorf("copy output = %q, want copied confirmation", out.String())
	}

	out.Reset()
	clip.err = context.Canceled
	copyText(context.Background(), mgr, &out, "43", "URL")
	if !strings.Contains(out.String(), "could not copy URL") {
		t.Errorf("error output = %q, want could not copy", out.String())
	}
}

func TestPrintKeyHelp(t *testing.T) {
	var out bytes.Buffer
	printKeyHelp(&out)
	for _, want := range []string{"kill gracefully", "open in browser", "show process tree", "quit"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("key help missing %q", want)
		}
	}
}
