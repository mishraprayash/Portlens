package cmd

import (
	"bytes"
	"context"
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
