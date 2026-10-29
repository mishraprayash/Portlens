//go:build linux

package platform

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
)

// OpenURL opens a URL in the default browser using xdg-open (or a fallback).
func OpenURL(ctx context.Context, url string) error {
	if err := validateOpenURL(url); err != nil {
		return err
	}
	for _, tool := range []string{"xdg-open", "sensible-browser", "x-www-browser"} {
		if path, err := exec.LookPath(tool); err == nil {
			cmd := exec.CommandContext(ctx, path, url)
			return cmd.Start()
		}
	}
	return fmt.Errorf("no browser opener available (install xdg-open)")
}

func validateOpenURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid URL %q: only http/https URLs are supported", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid URL %q: missing host", raw)
	}
	return nil
}
