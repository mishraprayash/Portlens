package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

func TestRunScanWritesLogViaTee(t *testing.T) {
	inUse, idle := runScanTest(t)
	logPath := filepath.Join(t.TempDir(), "scan.log")

	var out, errBuf bytes.Buffer
	w, f, err := teeLog(&out, logPath)
	if err != nil {
		t.Fatalf("teeLog: %v", err)
	}
	defer f.Close()
	opts := &options{ports: []int32{inUse, idle}, protocol: "tcp", noRecord: true, sortBy: "port"}
	code := runScan(context.Background(), w, &errBuf, opts)
	if code != exitcode.Success {
		t.Fatalf("runScan returned %d, want 0 (stderr: %s)", code, errBuf.String())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		fmt.Sprintf("Scanning 2 ports (%d-%d)...", inUse, idle),
		fmt.Sprintf("%d", inUse),
		fmt.Sprintf("Found "),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Scanning 2/2") {
		t.Errorf("progress should not be logged (it goes to stderr):\n%s", got)
	}
}

func TestTeeLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "out.log")
	var buf bytes.Buffer
	w, f, err := teeLog(&buf, logPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(w, "hello")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello\n" {
		t.Errorf("tee target output = %q, want %q", buf.String(), "hello\n")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" {
		t.Errorf("log file = %q, want %q", string(data), "hello\n")
	}
}

func TestTeeLogError(t *testing.T) {
	_, _, err := teeLog(io.Discard, filepath.Join(t.TempDir(), "no-such-dir", "out.log"))
	if err == nil {
		t.Error("expected error for unwritable log path")
	}
}

func TestExecuteLogsOutput(t *testing.T) {
	inUse, _ := runScanTest(t)
	logPath := filepath.Join(t.TempDir(), "out.log")

	var out, errBuf bytes.Buffer
	code := Execute([]string{fmt.Sprint(inUse), "--no-record", "--log", logPath}, &out, &errBuf, strings.NewReader(""))
	if code != exitcode.Success {
		t.Fatalf("Execute returned %d, want 0 (stderr: %s)", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "logging output to "+logPath) {
		t.Errorf("stderr should announce the log file:\n%s", errBuf.String())
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), fmt.Sprintf("PORT %d", inUse)) {
		t.Errorf("log should contain the report output:\n%s", string(data))
	}
}

func TestRunPortsJSONOnlyInUse(t *testing.T) {
	inUse, idle := runScanTest(t)

	var out, errBuf bytes.Buffer
	opts := &options{ports: []int32{inUse, idle}, protocol: "tcp", jsonOut: true, noRecord: true}
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
