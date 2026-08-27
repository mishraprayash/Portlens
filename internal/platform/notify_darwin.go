//go:build darwin

package platform

import (
	"context"
	"os/exec"
	"strings"
)

// Notify posts a desktop notification using osascript's display notification.
// It is best-effort: failures (for example, osascript missing) are returned so
// callers can report them without crashing.
func Notify(ctx context.Context, title, message string) error {
	script := "display notification " + appleScriptString(message) + " with title " + appleScriptString(title)
	return exec.CommandContext(ctx, "osascript", "-e", script).Run()
}

// appleScriptString quotes a value as an AppleScript string literal. The value
// is passed as a single argument to `osascript -e` (never through a shell), so
// only quotes and backslashes need escaping.
func appleScriptString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
