package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/portlens/portlens/internal/exitcode"
)

func TestRunFreeAlreadyFree(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFree(context.Background(), []string{"59999"}, &stdout, &stderr, nil)
	if code != exitcode.Success {
		t.Fatalf("runFree returned %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Port 59999 is already free") {
		t.Errorf("stdout = %q, want 'Port 59999 is already free'", stdout.String())
	}
}

func TestRunFreeNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFree(context.Background(), nil, &stdout, &stderr, nil)
	if code != exitcode.InvalidArguments {
		t.Fatalf("runFree returned %d, want %d", code, exitcode.InvalidArguments)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("expected usage on stderr, got: %s", stderr.String())
	}
}

func TestRunFreeLivePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind test listener: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Close listener before running free test to avoid killing test runner itself
	_ = ln.Close()

	var stdout, stderr bytes.Buffer
	code := runFree(context.Background(), []string{fmt.Sprintf("%d", port), "--yes"}, &stdout, &stderr, nil)
	if code != exitcode.Success {
		t.Fatalf("runFree returned %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "already free") {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}
