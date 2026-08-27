//go:build darwin

package platform

import (
	"context"
	"os/exec"
)

// OpenURL opens a URL in the default browser using macOS's `open` command.
func OpenURL(ctx context.Context, url string) error {
	return exec.CommandContext(ctx, "open", url).Run()
}
