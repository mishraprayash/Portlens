package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
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
	opts := &options{ports: []int32{inUse, idle}, protocol: "tcp", noRecord: true, sortBy: "port"}
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

func TestRunScanWritesLogFile(t *testing.T) {
	inUse, idle := runScanTest(t)
	logPath := filepath.Join(t.TempDir(), "scan.log")

	var out, errBuf bytes.Buffer
	opts := &options{ports: []int32{inUse, idle}, protocol: "tcp", noRecord: true, sortBy: "port", logPath: logPath}
	code := runScan(context.Background(), &out, &errBuf, opts)
	if code != exitcode.Success {
		t.Fatalf("runScan returned %d, want 0 (stderr: %s)", code, errBuf.String())
	}
	if !strings.Contains(out.String(), fmt.Sprintf("Logged 1 result(s) to %s", logPath)) {
		t.Errorf("expected log confirmation, got:\n%s", out.String())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"# PortLens scan log",
		fmt.Sprintf("# Ports scanned: 2 (%d-%d)", inUse, idle),
		"# Ports in use: 1",
		fmt.Sprintf("===== Port %d =====", inUse),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, fmt.Sprintf("===== Port %d =====", idle)) {
		t.Errorf("idle port should not be logged:\n%s", got)
	}
}

func TestRunScanUnwritableLogFile(t *testing.T) {
	var out, errBuf bytes.Buffer
	opts := &options{
		ports:    []int32{3000, 3001},
		protocol: "tcp",
		logPath:  filepath.Join(t.TempDir(), "no-such-dir", "scan.log"),
	}
	code := runScan(context.Background(), &out, &errBuf, opts)
	if code != exitcode.GeneralError {
		t.Errorf("runScan returned %d, want %d", code, exitcode.GeneralError)
	}
	if !strings.Contains(errBuf.String(), "cannot create log file") {
		t.Errorf("stderr = %q, want create-log-file error", errBuf.String())
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
