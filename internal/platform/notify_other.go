//go:build !darwin && !linux

package platform

import (
	"context"
	"fmt"
)

// Notify is a best-effort no-op on platforms without a supported desktop
// notification mechanism.
func Notify(ctx context.Context, title, message string) error {
	return fmt.Errorf("desktop notifications are not supported on this platform")
}
