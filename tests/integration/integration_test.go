// Package integration contains end-to-end tests that exercise PortLens against
// controlled, self-spawned test processes rather than arbitrary processes on
// the developer's machine.
package integration

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// TestMain re-invokes the test binary as a controllable HTTP server helper when
// the PORTLENS_TEST_HELPER environment variable is set.
func TestMain(m *testing.M) {
	if os.Getenv("PORTLENS_TEST_HELPER") == "1" {
		runHelper()
		return
	}
	os.Exit(m.Run())
}

func runHelper() {
	port := os.Getenv("PORTLENS_TEST_PORT")
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = ln
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})}
	fmt.Println("READY")
	if err := srv.Serve(ln); err != nil {
		os.Exit(0)
	}
}

// freePort returns an available TCP port on loopback.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// spawnServer starts the helper in a temporary project directory and waits for
// it to report READY. It returns the port, the process PID, and a cleanup func.
func spawnServer(t *testing.T) (int, int32, func()) {
	t.Helper()

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"),
		[]byte(`{"name":"portlens-integration","dependencies":{"@nestjs/core":"^10"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "PORTLENS_TEST_HELPER=1", "PORTLENS_TEST_PORT="+fmt.Sprint(port))
	cmd.Dir = projectDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	ready := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if scanner.Text() == "READY" {
				close(ready)
				return
			}
		}
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("helper did not become ready in time")
	}

	cleanup := func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	return port, int32(cmd.Process.Pid), cleanup
}

func TestInspectLiveServer(t *testing.T) {
	port, pid, cleanup := spawnServer(t)
	defer cleanup()

	plat := platform.New()
	insp := inspector.New(plat)

	report, err := insp.Inspect(context.Background(), int32(port), model.ProtocolTCP)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if report.Status != "listening" {
		t.Fatalf("status = %q, want listening", report.Status)
	}
	if report.Process == nil {
		t.Fatal("expected a process")
	}
	if report.Process.PID != pid {
		t.Errorf("pid = %d, want %d", report.Process.PID, pid)
	}
	if report.Address != "127.0.0.1" {
		t.Errorf("address = %q, want 127.0.0.1", report.Address)
	}
	if report.Project == nil || report.Project.Name != "portlens-integration" {
		t.Errorf("project = %+v, want portlens-integration", report.Project)
	}
	if report.Exposure == nil || report.Exposure.Worst != model.RiskLow {
		t.Errorf("exposure = %+v, want LOW RISK", report.Exposure)
	}
}

func TestInspectPortNotFound(t *testing.T) {
	port := freePort(t)
	plat := platform.New()
	insp := inspector.New(plat)

	report, err := insp.Inspect(context.Background(), int32(port), model.ProtocolTCP)
	if !errors.Is(err, inspector.ErrPortNotFound) {
		t.Fatalf("err = %v, want ErrPortNotFound", err)
	}
	if report.Status != "not_listening" {
		t.Errorf("status = %q, want not_listening", report.Status)
	}
}

func TestKillLiveServer(t *testing.T) {
	port, pid, cleanup := spawnServer(t)
	defer cleanup()

	plat := platform.New()
	insp := inspector.New(plat)
	report, err := insp.Inspect(context.Background(), int32(port), model.ProtocolTCP)
	if err != nil {
		t.Fatal(err)
	}

	mgr := actions.NewManager(plat, os.Stdout, nil)
	if err := mgr.Kill(context.Background(), report, false); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if plat.Controller.IsAlive(context.Background(), pid) {
		t.Errorf("process %d still alive after graceful kill", pid)
	}
}

func TestProcessTree(t *testing.T) {
	port, pid, cleanup := spawnServer(t)
	defer cleanup()

	plat := platform.New()
	tree, err := plat.Tree.Descendants(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Process.PID != pid {
		t.Errorf("tree root pid = %d, want %d", tree.Process.PID, pid)
	}

	ancestors, err := plat.Tree.Ancestors(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(ancestors) == 0 {
		t.Fatal("expected at least one ancestor")
	}
	if ancestors[len(ancestors)-1].PID != pid {
		t.Errorf("last ancestor pid = %d, want %d (target)", ancestors[len(ancestors)-1].PID, pid)
	}

	_ = port
}
