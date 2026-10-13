//go:build darwin

package platform

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
)

// OpenURL opens a URL in the default browser using macOS's `open` command.
func OpenURL(ctx context.Context, url string) error {
	if err := validateOpenURL(url); err != nil {
		return err
	}
	return exec.CommandContext(ctx, "open", url).Run()
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
