//go:build linux

package platform

import (
	"context"
	"os/exec"
)

// Notify posts a desktop notification via notify-send. Failures (for example,
// notify-send missing in a headless environment) are returned to the caller.
func Notify(ctx context.Context, title, message string) error {
	return exec.CommandContext(ctx, "notify-send", title, message).Run()
}
