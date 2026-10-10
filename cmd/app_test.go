package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/portlens/portlens/internal/exitcode"
)

// runScanTest builds a scanner over a real bound port plus a second port that
// is very likely idle, so the scan can be exercised end to end.
func runScanTest(t *testing.T) (int32, int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind a test listener: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	inUse := int32(ln.Addr().(*net.TCPAddr).Port)

	idle := inUse + 1
	if tl, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", idle)); err == nil {
		_ = tl.Close()
	}
	return inUse, idle
}

func TestRunScanReportsOnlyInUsePorts(t *testing.T) {
	inUse, idle := runScanTest(t)

	var out, errBuf bytes.Buffer
	opts := &options{ports: []int32{inUse, idle}, protocol: "tcp", sortBy: "port"}
	code := runScan(context.Background(), &out, &errBuf, opts)
	if code != exitcode.Success {
		t.Fatalf("runScan returned %d, want 0 (stderr: %s)", code, errBuf.String())
	}

	got := out.String()
	for _, want := range []string{
		fmt.Sprintf("Scanning 2 ports (%d-%d)...", inUse, idle),
		fmt.Sprintf("%d", inUse),
		"Found ",
		" of 2 ports in use in ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRunPortsJSONOnlyInUse(t *testing.T) {
	inUse, idle := runScanTest(t)

	var out, errBuf bytes.Buffer
	opts := &options{ports: []int32{inUse, idle}, protocol: "tcp", jsonOut: true}
	code := runPortsJSON(context.Background(), &out, &errBuf, opts)
	if code != exitcode.Success {
		t.Fatalf("runPortsJSON returned %d, want 0 (stderr: %s)", code, errBuf.String())
	}

	got := out.String()
	if !strings.Contains(got, fmt.Sprintf("\"port\": %d", inUse)) {
		t.Errorf("JSON should include the in-use port %d:\n%s", inUse, got)
	}
	if strings.Contains(got, fmt.Sprintf("\"port\": %d", idle)) {
		t.Errorf("JSON should omit the idle port %d:\n%s", idle, got)
	}
	// stdout must be a pure JSON array.
	if !strings.HasPrefix(strings.TrimSpace(got), "[") {
		t.Errorf("stdout should be a JSON array:\n%s", got)
	}
	// progress + summary feedback on stderr.
	progress := errBuf.String()
	if !strings.Contains(progress, fmt.Sprintf("Scanning 2 ports (%d-%d)", inUse, idle)) ||
		!strings.Contains(progress, "Found 1 of 2 ports in use") {
		t.Errorf("stderr should carry scan feedback:\n%s", progress)
	}
}

func TestWriteScanProgress(t *testing.T) {
	var b bytes.Buffer
	writeScanProgress(&b, 100, 1000, 5*time.Second, 3, true)
	if !strings.Contains(b.String(), "\r") {
		t.Error("expected carriage-return progress line")
	}
	if !strings.Contains(b.String(), "100/1000 (10.0%)") || !strings.Contains(b.String(), "3 in use") || !strings.Contains(b.String(), "ETA") {
		t.Errorf("progress line = %q", b.String())
	}

	b.Reset()
	writeScanProgress(&b, 100, 1000, 5*time.Second, 3, false)
	if !strings.HasSuffix(b.String(), "\n") || strings.Contains(b.String(), "\r") {
		t.Errorf("non-interactive progress line = %q", b.String())
	}
}

func TestRunScanMultipleActivePortsParallel(t *testing.T) {
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind listener: %v", err)
	}
	defer ln1.Close()
	p1 := int32(ln1.Addr().(*net.TCPAddr).Port)

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind listener: %v", err)
	}
	defer ln2.Close()
	p2 := int32(ln2.Addr().(*net.TCPAddr).Port)

	idle1 := int32(64001)
	idle2 := int32(64002)

	var out, errBuf bytes.Buffer
	opts := &options{
		ports:    []int32{p1, idle1, p2, idle2},
		protocol: "tcp",
		sortBy:   "port",
	}
	code := runScan(context.Background(), &out, &errBuf, opts)
	if code != exitcode.Success {
		t.Fatalf("runScan returned %d, want 0 (stderr: %s)", code, errBuf.String())
	}

	got := out.String()
	if !strings.Contains(got, fmt.Sprintf("%d", p1)) {
		t.Errorf("missing port %d in output: %s", p1, got)
	}
	if !strings.Contains(got, fmt.Sprintf("%d", p2)) {
		t.Errorf("missing port %d in output: %s", p2, got)
	}
	if !strings.Contains(got, "Found 2 of 4 ports in use") {
		t.Errorf("expected 2 of 4 ports in use, got: %s", got)
	}
}
