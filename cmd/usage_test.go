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
