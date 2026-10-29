package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewDisablesColorOnNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	if r.Color {
		t.Fatal("color enabled for a non-terminal writer")
	}
	r.writeln(r.bold("x"))
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("escape sequences in piped output: %q", buf.String())
	}
}

func TestNoColorEnvWinsOverWithColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if r := NewRenderer(&bytes.Buffer{}, WithColor(true)); r.Color {
		t.Fatal("NO_COLOR ignored")
	}
}
