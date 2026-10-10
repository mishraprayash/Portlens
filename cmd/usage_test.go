package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintUsage(t *testing.T) {
	var out bytes.Buffer
	printUsage(&out)
	for _, want := range []string{"PortLens", "COMMANDS", "EXIT CODES", "Manage named port groups"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("usage missing %q", want)
		}
	}
}

// TestHelpListsEveryCommand guards against help/registry drift: a newly
// registered subcommand must show up in `portlens --help`.
func TestHelpListsEveryCommand(t *testing.T) {
	var out bytes.Buffer
	printUsage(&out)
	help := out.String()
	for _, c := range defaultSubcommandRegistry().ordered {
		if !strings.Contains(help, c.Name()) {
			t.Errorf("help text missing registered command %q", c.Name())
		}
	}
}
