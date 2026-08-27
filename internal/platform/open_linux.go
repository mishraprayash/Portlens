//go:build linux

package platform

import (
	"context"
	"fmt"
	"os/exec"
)

// OpenURL opens a URL in the default browser using xdg-open (or a fallback).
func OpenURL(ctx context.Context, url string) error {
	for _, tool := range []string{"xdg-open", "sensible-browser", "x-www-browser"} {
		if path, err := exec.LookPath(tool); err == nil {
			return exec.CommandContext(ctx, path, url).Run()
		}
	}
	return fmt.Errorf("no browser opener available (install xdg-open)")
}
