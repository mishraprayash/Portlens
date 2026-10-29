// Package version holds the PortLens version string. Release builds stamp it
// via -ldflags "-X github.com/mishraprayash/Portlens/internal/version.Version=...".
// Builds made with `go install ...@vX.Y.Z` have no stamp and fall back to the
// module version embedded by the Go toolchain.
package version

import (
	"runtime/debug"
	"strings"
)

// Version is the semantic version of this build ("dev" for unstamped local builds).
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		Version = fromBuildInfo(info.Main.Version, Version)
	}
}

// fromBuildInfo returns the module version without its leading "v", or
// fallback when the toolchain recorded no real version (local builds report
// "(devel)" or an empty string).
func fromBuildInfo(moduleVersion, fallback string) string {
	if moduleVersion == "" || moduleVersion == "(devel)" {
		return fallback
	}
	return strings.TrimPrefix(moduleVersion, "v")
}
