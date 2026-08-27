package cmd

import (
	"reflect"
	"testing"
)

func TestReorderArgs(t *testing.T) {
	cases := []struct {
		in   []string
		want argSplit
	}{
		{[]string{"3000", "--tree"}, argSplit{flags: []string{"--tree"}, positional: []string{"3000"}}},
		{[]string{"--tree", "3000"}, argSplit{flags: []string{"--tree"}, positional: []string{"3000"}}},
		{[]string{"--json"}, argSplit{flags: []string{"--json"}}},
		{[]string{"3000", "--protocol", "tcp"}, argSplit{flags: []string{"--protocol", "tcp"}, positional: []string{"3000"}}},
		{[]string{"3000", "--sort=process"}, argSplit{flags: []string{"--sort=process"}, positional: []string{"3000"}}},
		{[]string{}, argSplit{}},
	}
	for _, c := range cases {
		got := reorderArgs(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("reorderArgs(%v) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseArgsFlagsAfterPort(t *testing.T) {
	opts, err := parseArgs([]string{"3000", "--tree", "--no-color"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.port != 3000 || !opts.tree || !opts.noColor {
		t.Errorf("opts = %+v", opts)
	}
}

func TestParseArgsInvalidPort(t *testing.T) {
	if _, err := parseArgs([]string{"abc"}); err == nil {
		t.Error("expected error for invalid port")
	}
	if _, err := parseArgs([]string{"70000"}); err == nil {
		t.Error("expected error for out-of-range port")
	}
	if _, err := parseArgs([]string{"3000", "3001"}); err == nil {
		t.Error("expected error for too many arguments")
	}
}

func TestParseArgsForceRequiresKill(t *testing.T) {
	if _, err := parseArgs([]string{"3000", "--force"}); err == nil {
		t.Error("expected error for --force without --kill")
	}
}
