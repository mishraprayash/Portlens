package cmd

import (
	"errors"
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

func TestParsePortArg(t *testing.T) {
	cases := []struct {
		in   string
		want []int32
		err  bool
	}{
		{"3000", []int32{3000}, false},
		{"3000-3005", []int32{3000, 3001, 3002, 3003, 3004, 3005}, false},
		{"3000:3002", []int32{3000, 3001, 3002}, false},
		{"65535", []int32{65535}, false},
		{"abc", nil, true},
		{"70000", nil, true},
		{"0", nil, true},
		{"3000-2000", nil, true},
		{"3000-", nil, true},
		{"-3000", nil, true},
		{"1-70000", nil, true},
	}
	for _, c := range cases {
		got, err := parsePortArg(c.in)
		if c.err {
			if err == nil {
				t.Errorf("parsePortArg(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePortArg(%q): unexpected error %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parsePortArg(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParsePortArgTooLargeRange(t *testing.T) {
	if _, err := parsePortArg("1-99999"); err == nil {
		t.Error("expected error for oversized range")
	}
}

func TestParseArgsRange(t *testing.T) {
	opts, err := parseArgs([]string{"3000-3003", "--tree"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opts.ports, []int32{3000, 3001, 3002, 3003}) {
		t.Errorf("ports = %v", opts.ports)
	}
	if !opts.tree {
		t.Error("expected --tree")
	}
}

func TestParseArgsAll(t *testing.T) {
	opts, err := parseArgs([]string{"--all"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.all {
		t.Error("expected --all to be set")
	}
	if _, err := parseArgs([]string{"3000", "--all"}); err == nil {
		t.Error("expected error combining --all with explicit ports")
	}
}

func TestParseArgsPidName(t *testing.T) {
	if _, err := parseArgs([]string{"--pid", "0"}); err == nil {
		t.Error("expected error for --pid 0")
	}
	if _, err := parseArgs([]string{"--pid", "-1"}); err == nil {
		t.Error("expected error for --pid -1")
	}
	if _, err := parseArgs([]string{"--name", "   "}); err == nil {
		t.Error("expected error for blank --name")
	}
	if _, err := parseArgs([]string{"3000", "--pid", "123"}); err == nil {
		t.Error("expected error combining --pid with explicit ports")
	}
	if _, err := parseArgs([]string{"3000", "--name", "node"}); err == nil {
		t.Error("expected error combining --name with explicit ports")
	}
	opts, err := parseArgs([]string{"--name", "node"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.name != "node" {
		t.Errorf("name = %q", opts.name)
	}
}

func TestParseArgsWatchValidation(t *testing.T) {
	if _, err := parseArgs([]string{"3000", "--notify"}); err == nil {
		t.Error("expected error: --notify requires --watch")
	}
	if _, err := parseArgs([]string{"3000", "--interval", "2"}); err == nil {
		t.Error("expected error: --interval requires --watch")
	}
	if _, err := parseArgs([]string{"3000", "--interval", "0", "--watch"}); err == nil {
		t.Error("expected error: interval must be positive")
	}
	opts, err := parseArgs([]string{"3000", "--watch", "--notify", "--interval", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.watch || !opts.notify || opts.interval != 2 {
		t.Errorf("opts = %+v", opts)
	}
}

func TestExpandGroups(t *testing.T) {
	lookup := func(name string) ([]int32, error) {
		switch name {
		case "dev":
			return []int32{3000, 4000}, nil
		case "web":
			return []int32{8080}, nil
		default:
			return nil, errors.New("not found")
		}
	}
	cases := []struct {
		in   []string
		want []string
		err  bool
	}{
		{[]string{"@dev"}, []string{"3000", "4000"}, false},
		{[]string{"@dev", "--tree"}, []string{"3000", "4000", "--tree"}, false},
		{[]string{"--protocol", "udp", "@web"}, []string{"--protocol", "udp", "8080"}, false},
		{[]string{"--name", "@foo"}, []string{"--name", "@foo"}, false},
		{[]string{"@nope"}, nil, true},
		{[]string{}, nil, false},
	}
	for _, c := range cases {
		got, err := expandGroups(c.in, lookup)
		if c.err {
			if err == nil {
				t.Errorf("expandGroups(%v): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("expandGroups(%v): %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("expandGroups(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFormatPorts(t *testing.T) {
	cases := []struct {
		in   []int32
		want string
	}{
		{[]int32{3000}, "3000"},
		{[]int32{3000, 3001, 3002}, "3000-3002"},
		{[]int32{3000, 3002, 4000, 4001, 4002}, "3000, 3002, 4000-4002"},
		{[]int32{5000, 1000, 1001, 1002}, "1000-1002, 5000"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := formatPorts(c.in); got != c.want {
			t.Errorf("formatPorts(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseArgsForceRequiresKill(t *testing.T) {
	if _, err := parseArgs([]string{"3000", "--force"}); err == nil {
		t.Error("expected error for --force without --kill")
	}
}
