//go:build linux

package platform

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// linuxClipboardProvider copies text to the clipboard using whichever of the
// common Linux clipboard tools is available (wl-copy for Wayland, xclip/xsel
// for X11). It fails gracefully when none are installed.
type linuxClipboardProvider struct{}

func newClipboardProvider() ClipboardProvider { return linuxClipboardProvider{} }

var linuxClipboardCandidates = []struct {
	cmd  string
	args func(text string) []string
}{
	{"wl-copy", func(text string) []string { return nil }}, // reads stdin
	{"xclip", func(text string) []string { return []string{"-selection", "clipboard"} }},
	{"xsel", func(text string) []string { return []string{"--clipboard", "--input"} }},
}

func (linuxClipboardProvider) Copy(ctx context.Context, text string) error {
	var lastErr error
	for _, c := range linuxClipboardCandidates {
		path, err := exec.LookPath(c.cmd)
		if err != nil {
			continue
		}
		_ = path
		cmd := exec.CommandContext(ctx, c.cmd, c.args(text)...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no clipboard tool available (install wl-copy, xclip, or xsel)")
	}
	return lastErr
}
