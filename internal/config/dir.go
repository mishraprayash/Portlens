// Package config manages PortLens's user configuration file. The current
// configuration is limited to named port groups (e.g. `portlens @dev` expands
// to a predefined set of ports), but the file format is forward-compatible.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Dir returns the platform-appropriate data directory for PortLens,
// respecting XDG_DATA_HOME on Linux.
func Dir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "portlens")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "portlens")
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "portlens")
		}
		return filepath.Join(home, ".portlens")
	default:
		return filepath.Join(home, ".local", "share", "portlens")
	}
}

// Path returns the location of the configuration file.
func Path() string {
	return filepath.Join(Dir(), "config.json")
}
