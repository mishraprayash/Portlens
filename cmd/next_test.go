package cmd

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/portlens/portlens/internal/exitcode"
)

func TestRunNextDefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runNext(context.Background(), nil, &stdout, &stderr)
	if code != exitcode.Success {
		t.Fatalf("runNext returned %d, want 0 (stderr: %s)", code, stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	p, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("runNext output %q is not a number", out)
	}
	if p < 3000 || p > 65535 {
		t.Errorf("expected port in 3000..65535, got %d", p)
	}
}

func TestRunNextStartPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runNext(context.Background(), []string{"8500"}, &stdout, &stderr)
	if code != exitcode.Success {
		t.Fatalf("runNext returned %d, want 0 (stderr: %s)", code, stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	p, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("runNext output %q is not a number", out)
	}
	if p < 8500 || p > 65535 {
		t.Errorf("expected port >= 8500, got %d", p)
	}
}

func TestRunNextInvalidPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runNext(context.Background(), []string{"99999"}, &stdout, &stderr)
	if code != exitcode.InvalidArguments {
		t.Fatalf("runNext returned %d, want %d", code, exitcode.InvalidArguments)
	}
}
