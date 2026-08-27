package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Config holds user configuration. The only section today is named port
// groups: `portlens @<group>` expands to the group's ports.
type Config struct {
	Groups map[string][]int32 `json:"groups,omitempty"`
}

// Load reads the configuration file, returning an empty configuration when the
// file does not exist yet. A corrupt file is surfaced as an error rather than
// silently ignored.
func Load() (*Config, error) {
	return LoadAt(Path())
}

// LoadAt reads the configuration from an explicit path (used by tests).
func LoadAt(path string) (*Config, error) {
	c := &Config{Groups: map[string][]int32{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	if c.Groups == nil {
		c.Groups = map[string][]int32{}
	}
	return c, nil
}

// Save writes the configuration atomically (temp file + rename) with
// owner-only permissions.
func (c *Config) Save() error {
	return c.SaveAt(Path())
}

// SaveAt writes the configuration to an explicit path (used by tests).
func (c *Config) SaveAt(path string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ErrInvalidName is returned when a group name is not usable.
var ErrInvalidName = errors.New("invalid group name")

// GroupNames returns all group names, sorted.
func (c *Config) GroupNames() []string {
	names := make([]string, 0, len(c.Groups))
	for n := range c.Groups {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Ports returns a group's ports and whether the group exists.
func (c *Config) Ports(name string) ([]int32, bool) {
	p, ok := c.Groups[name]
	return p, ok
}

// SetGroup creates or replaces a group. The name must be non-empty and contain
// no whitespace, and the group must have at least one port.
func (c *Config) SetGroup(name string, ports []int32) error {
	if !validName(name) {
		return fmt.Errorf("%w: %q (must be non-empty and contain no whitespace)", ErrInvalidName, name)
	}
	if len(ports) == 0 {
		return fmt.Errorf("group %q must contain at least one port", name)
	}
	c.Groups[name] = append([]int32(nil), ports...)
	return nil
}

// RemoveGroup deletes a group and reports whether it existed.
func (c *Config) RemoveGroup(name string) bool {
	if _, ok := c.Groups[name]; !ok {
		return false
	}
	delete(c.Groups, name)
	return true
}

func validName(name string) bool {
	return name != "" && !strings.ContainsAny(name, " \t\n\r")
}
