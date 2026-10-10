package cmd

import (
	"bytes"
	"context"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/mishraprayash/Portlens/internal/exitcode"
)

func TestRunCompletionBash(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCompletion([]string{"bash"}, &stdout, &stderr)
	if code != exitcode.Success {
		t.Fatalf("runCompletion bash returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "_portlens_completions") {
		t.Errorf("bash script missing _portlens_completions")
	}
}

func TestRunCompletionZsh(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCompletion([]string{"zsh"}, &stdout, &stderr)
	if code != exitcode.Success {
		t.Fatalf("runCompletion zsh returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "#compdef portlens") {
		t.Errorf("zsh script missing #compdef portlens")
	}
}

func TestRunCompletionFish(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCompletion([]string{"fish"}, &stdout, &stderr)
	if code != exitcode.Success {
		t.Fatalf("runCompletion fish returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "complete -c portlens") {
		t.Errorf("fish script missing complete -c portlens")
	}
}

func TestRunCompletionInvalidShell(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCompletion([]string{"powershell"}, &stdout, &stderr)
	if code != exitcode.InvalidArguments {
		t.Fatalf("runCompletion returned %d, want %d", code, exitcode.InvalidArguments)
	}
}

func TestRunCompletePorts(t *testing.T) {
	var stdout bytes.Buffer
	code := runCompletePorts(context.Background(), &stdout)
	if code != exitcode.Success {
		t.Fatalf("runCompletePorts returned %d, want 0", code)
	}
}

// Completion must never fail the shell: a canceled or expired context
// yields no completions but still exits 0.
func TestRunCompletePortsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout bytes.Buffer
	if code := runCompletePorts(ctx, &stdout); code != exitcode.Success {
		t.Fatalf("runCompletePorts with canceled context returned %d, want 0", code)
	}
}

func TestExecuteCompletionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"completion", "zsh"}, &stdout, &stderr, nil)
	if code != exitcode.Success {
		t.Fatalf("Execute completion zsh returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "#compdef portlens") {
		t.Errorf("expected zsh completion script")
	}
}

// completionSegment extracts the quoted value that follows marker in script.
func completionSegment(script, marker string) (string, bool) {
	start := strings.Index(script, marker)
	if start < 0 {
		return "", false
	}
	start += len(marker)
	end := strings.IndexByte(script[start:], '"')
	if end < 0 {
		return "", false
	}
	return script[start : start+end], true
}

// commandAnchored reports whether the generated script offers name as a
// completable subcommand (using each shell's exact syntax, so substring
// collisions with descriptions or the config sub-actions cannot pass).
func commandAnchored(script, shell, name string) bool {
	switch shell {
	case "bash":
		segment, ok := completionSegment(script, `local subcmds="`)
		if !ok {
			return false
		}
		for _, w := range strings.Fields(segment) {
			if w == name {
				return true
			}
		}
		return false
	case "zsh":
		return strings.Contains(script, "'"+name+":")
	case "fish":
		return strings.Contains(script, "-a '"+name+"'")
	}
	return false
}

// flagAnchored reports whether the generated script offers flag f.
func flagAnchored(script, shell string, f flagDoc) bool {
	switch shell {
	case "bash":
		segment, ok := completionSegment(script, `local flags="`)
		if !ok {
			return false
		}
		for _, w := range strings.Fields(segment) {
			if w == "--"+f.long || (f.short != "" && w == "-"+f.short) {
				return true
			}
		}
		return false
	case "zsh":
		if strings.Contains(script, "'--"+f.long+"[") {
			return true
		}
		return f.short != "" && strings.Contains(script, "'-"+f.short+"[")
	case "fish":
		if strings.Contains(script, "-l "+f.long+" ") {
			return true
		}
		return f.short != "" && strings.Contains(script, "-s "+f.short+" ")
	}
	return false
}

// TestCompletionScriptsCoverRegistry guards against script drift: every
// registered command and alias must be offered by every shell, removed
// aliases must not reappear, and no template token may leak into output.
func TestCompletionScriptsCoverRegistry(t *testing.T) {
	reg := defaultSubcommandRegistry()
	removed := []string{"info", "show", "stop", "term", "net", "search", "free", "dashboard"}
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var out bytes.Buffer
		if code := runCompletion([]string{shell}, &out, io.Discard); code != exitcode.Success {
			t.Fatalf("runCompletion %s returned %d, want 0", shell, code)
		}
		script := out.String()
		if strings.Contains(script, "{{") {
			t.Errorf("%s script contains an unrendered template token", shell)
		}
		for _, c := range reg.ordered {
			for _, n := range append([]string{c.Name()}, c.Aliases()...) {
				if !commandAnchored(script, shell, n) {
					t.Errorf("%s completion missing registered command %q", shell, n)
				}
			}
		}
		for _, gone := range removed {
			if commandAnchored(script, shell, gone) {
				t.Errorf("%s completion offers removed alias %q", shell, gone)
			}
		}
	}
}

// TestCompletionScriptsCoverFlags asserts every flag in flagDocs is offered
// by every shell's script (both long and short forms).
func TestCompletionScriptsCoverFlags(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var out bytes.Buffer
		if code := runCompletion([]string{shell}, &out, io.Discard); code != exitcode.Success {
			t.Fatalf("runCompletion %s returned %d, want 0", shell, code)
		}
		script := out.String()
		for _, f := range flagDocs {
			if !flagAnchored(script, shell, f) {
				t.Errorf("%s completion missing flag --%s", shell, f.long)
			}
		}
	}
}

// TestFlagDocsMatchRegisteredFlags asserts the completion flag table and the
// flags parseArgs actually registers are the same set, in both directions.
func TestFlagDocsMatchRegisteredFlags(t *testing.T) {
	fs := flag.NewFlagSet("portlens", flag.ContinueOnError)
	registerFlags(fs, &options{})
	defined := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) { defined[f.Name] = true })

	documented := map[string]bool{}
	for _, d := range flagDocs {
		if documented[d.long] {
			t.Errorf("duplicate flagDocs entry for --%s", d.long)
		}
		documented[d.long] = true
		if d.short != "" {
			documented[d.short] = true
		}
	}
	for name := range defined {
		if !documented[name] {
			t.Errorf("flag %q is registered but missing from flagDocs", name)
		}
	}
	for name := range documented {
		if !defined[name] {
			t.Errorf("flag %q is in flagDocs but not registered", name)
		}
	}
}
