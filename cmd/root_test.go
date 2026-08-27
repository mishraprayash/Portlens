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
	if !reflect.DeepEqual(opts.ports, []int32{3000}) || !opts.tree || !opts.noColor {
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
}

func TestParseArgsMultiplePorts(t *testing.T) {
	opts, err := parseArgs([]string{"3000", "4000", "5000", "--tree"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opts.ports, []int32{3000, 4000, 5000}) {
		t.Errorf("ports = %v, want [3000 4000 5000]", opts.ports)
	}
	if !opts.tree {
		t.Error("expected --tree")
	}
}

func TestParseArgsMultiplePortsFlagsBetween(t *testing.T) {
	opts, err := parseArgs([]string{"3000", "--protocol", "udp", "4000"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opts.ports, []int32{3000, 4000}) {
		t.Errorf("ports = %v, want [3000 4000]", opts.ports)
	}
	if opts.protocol != "udp" {
		t.Errorf("protocol = %q, want udp", opts.protocol)
	}
}

func TestParseArgsDedupePorts(t *testing.T) {
	opts, err := parseArgs([]string{"3000", "3000", "4000"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opts.ports, []int32{3000, 4000}) {
		t.Errorf("ports = %v, want [3000 4000]", opts.ports)
	}
}

func TestDedupePorts(t *testing.T) {
	in := []int32{8080, 3000, 8080, 4000, 3000}
	got := dedupePorts(in)
	want := []int32{8080, 3000, 4000}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupePorts(%v) = %v, want %v", in, got, want)
	}
}

func TestParseArgsForceRequiresKill(t *testing.T) {
	if _, err := parseArgs([]string{"3000", "--force"}); err == nil {
		t.Error("expected error for --force without --kill")
	}
}
