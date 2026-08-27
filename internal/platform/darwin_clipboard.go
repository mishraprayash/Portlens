//go:build darwin

package platform

import (
	"context"
	"os/exec"
)

// darwinClipboardProvider copies text via pbcopy.
type darwinClipboardProvider struct{}

func newClipboardProvider() ClipboardProvider { return darwinClipboardProvider{} }

func (darwinClipboardProvider) Copy(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "pbcopy")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}
