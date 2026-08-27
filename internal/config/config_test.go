package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAtMissingFile(t *testing.T) {
	c, err := LoadAt(filepath.Join(t.TempDir(), "nope", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Groups) != 0 {
		t.Errorf("expected empty config, got %v", c.Groups)
	}
}

func TestLoadAtEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeConfig(t, path, "")
	c, err := LoadAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Groups) != 0 {
		t.Errorf("expected empty config, got %v", c.Groups)
	}
}

func TestLoadAtCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeConfig(t, path, "{not json")
	if _, err := LoadAt(path); err == nil {
		t.Error("expected error for corrupt config")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c := &Config{Groups: map[string][]int32{}}
	if err := c.SetGroup("dev", []int32{3000, 4000, 5000}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetGroup("web", []int32{8080}); err != nil {
		t.Fatal(err)
	}
	if err := c.SaveAt(path); err != nil {
		t.Fatal(err)
	}

	got, err := LoadAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Groups, c.Groups) {
		t.Errorf("round trip = %v, want %v", got.Groups, c.Groups)
	}
}

func TestSetGroupValidation(t *testing.T) {
	c := &Config{Groups: map[string][]int32{}}
	if err := c.SetGroup("", []int32{3000}); err == nil {
		t.Error("expected error for empty name")
	}
	if err := c.SetGroup("my group", []int32{3000}); err == nil {
		t.Error("expected error for name with whitespace")
	}
	if err := c.SetGroup("dev", nil); err == nil {
		t.Error("expected error for empty ports")
	}
}

func TestRemoveGroup(t *testing.T) {
	c := &Config{Groups: map[string][]int32{"dev": {3000}}}
	if c.RemoveGroup("missing") {
		t.Error("expected false for missing group")
	}
	if !c.RemoveGroup("dev") {
		t.Error("expected true for existing group")
	}
	if _, ok := c.Ports("dev"); ok {
		t.Error("group should be gone")
	}
}

func TestGroupNamesSorted(t *testing.T) {
	c := &Config{Groups: map[string][]int32{"z": {1}, "a": {2}, "m": {3}}}
	got := c.GroupNames()
	want := []string{"a", "m", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupNames() = %v, want %v", got, want)
	}
}
