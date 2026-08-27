package render

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/portlens/portlens/internal/model"
)

func reportNow() time.Time { return time.Now() }

// containerLabel renders a container as "name (image)" or "name (image, svc)"
// when the service name is known, falling back to the container ID when the
// runtime-assigned name is unavailable.
func containerLabel(c *model.Container) string {
	if c == nil {
		return ""
	}
	name := c.Name
	if name == "" {
		name = c.ID
	}
	image := c.Image
	if image == "" {
		image = c.Status
	}
	label := name
	if image != "" {
		label = fmt.Sprintf("%s (%s", name, image)
		if c.ComposeService != "" {
			label += ", " + c.ComposeService
		}
		label += ")"
	}
	return strings.TrimSpace(label)
}

var cachedHome = os.Getenv("HOME")

func userHomeDir() string {
	if cachedHome != "" {
		return cachedHome
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	cachedHome = home
	return home
}
