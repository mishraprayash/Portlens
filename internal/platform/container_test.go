package platform

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/portlens/portlens/internal/model"
)

func TestContainerIDFromCgroup(t *testing.T) {
	id := strings.Repeat("a", 64)
	id2 := strings.Repeat("b", 64)
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"cgroup v1 docker", "12:memory:/docker/" + id, id},
		{"cgroup v2 systemd scope", "0::/system.slice/docker-" + id + ".scope", id},
		{"kubepods cri-containerd", "5:cpu,cpuacct:/kubepods/burstable/pod123/cri-containerd-" + id2 + ".scope", id2},
		{"not in a container", "12:blkio:/", ""},
		{"empty", "", ""},
		{"embedded in longer hex", id + "a", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := containerIDFromCgroup(tc.in); got != tc.want {
				t.Errorf("containerIDFromCgroup(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDockerSocketPaths(t *testing.T) {
	t.Run("honors unix DOCKER_HOST first", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "unix:///tmp/alt.sock")
		paths := dockerSocketPaths()
		if len(paths) == 0 || paths[0] != "/tmp/alt.sock" {
			t.Fatalf("first path = %v, want /tmp/alt.sock", paths)
		}
	})
	t.Run("ignores non-unix DOCKER_HOST", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "tcp://127.0.0.1:2375")
		paths := dockerSocketPaths()
		for _, p := range paths {
			if strings.Contains(p, "2375") {
				t.Fatalf("non-unix DOCKER_HOST leaked into paths: %v", paths)
			}
		}
		if paths[0] != "/var/run/docker.sock" {
			t.Fatalf("first path = %q, want /var/run/docker.sock", paths[0])
		}
	})
}

func TestContainerFromSummary(t *testing.T) {
	s := containerSummary{
		ID:    strings.Repeat("c", 64),
		Names: []string{"/api-1"},
		Image: "nginx:alpine",
		State: "running",
		Labels: map[string]string{
			composeProjectLabel: "orbit",
			composeServiceLabel: "api",
		},
	}
	c := containerFromSummary(s)
	if c.Name != "api-1" || c.Image != "nginx:alpine" || c.Status != "running" {
		t.Fatalf("unexpected container: %+v", c)
	}
	if c.ComposeProject != "orbit" || c.ComposeService != "api" {
		t.Fatalf("compose labels not mapped: %+v", c)
	}
}

// startDockerServer serves the given handler over a unix socket and returns
// the socket path, so the dockerProvider's real transport is exercised. The
// path is kept short because macOS limits unix socket paths (sun_path) to 104
// bytes and long t.TempDir() paths exceed that.
func startDockerServer(t *testing.T, h http.Handler) string {
	t.Helper()
	sock := filepath.Join(os.TempDir(), fmt.Sprintf("pl-test-%d.sock", sockSeq.Add(1)))
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("unix listen: %v", err)
	}
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = ln.Close(); _ = srv.Close(); _ = os.Remove(sock) })
	return sock
}

var sockSeq atomic.Int64

func testContainerJSON() string {
	return fmt.Sprintf(`[
	  {"Id": %q, "Names": ["/api-1"], "Image": "nginx:alpine", "State": "running",
	   "Labels": {"com.docker.compose.project": "orbit", "com.docker.compose.service": "api"},
	   "Ports": [{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"}]},
	  {"Id": %q, "Names": ["/redis-1"], "Image": "redis:7", "State": "running",
	   "Ports": [{"IP": "0.0.0.0", "PrivatePort": 6379, "PublicPort": 6379, "Type": "tcp"}]}
	]`, strings.Repeat("a", 64), strings.Repeat("b", 64))
}

func TestFindByPorts(t *testing.T) {
	sock := startDockerServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, testContainerJSON())
	}))
	d := newDockerProvider(sock)

	ctx := context.Background()
	got, err := d.FindByPorts(ctx, []uint16{8080, 6379, 9999}, model.ProtocolTCP)
	if err != nil {
		t.Fatalf("FindByPorts: %v", err)
	}
	if c := got[8080]; c == nil || c.Name != "api-1" || c.ComposeService != "api" {
		t.Errorf("8080 -> %+v, want api-1", c)
	}
	if c := got[6379]; c == nil || c.Name != "redis-1" {
		t.Errorf("6379 -> %+v, want redis-1", c)
	}
	if _, ok := got[9999]; ok {
		t.Errorf("9999 should not map to a container: %+v", got)
	}

	if _, err := d.FindByPorts(ctx, []uint16{8080}, model.ProtocolUDP); err != nil {
		t.Fatalf("FindByPorts udp: %v", err)
	}
	if _, err := d.FindByPorts(ctx, nil, model.ProtocolTCP); err != nil {
		t.Fatalf("FindByPorts empty: %v", err)
	}
}

func TestFindByPort(t *testing.T) {
	sock := startDockerServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testContainerJSON())
	}))
	d := newDockerProvider(sock)

	c, err := d.FindByPort(context.Background(), 8080, model.ProtocolTCP)
	if err != nil {
		t.Fatalf("FindByPort: %v", err)
	}
	if c == nil || c.Image != "nginx:alpine" {
		t.Fatalf("FindByPort = %+v, want nginx container", c)
	}

	c, err = d.FindByPort(context.Background(), 9999, model.ProtocolTCP)
	if err != nil {
		t.Fatalf("FindByPort: %v", err)
	}
	if c != nil {
		t.Fatalf("FindByPort(9999) = %+v, want nil", c)
	}
}

func TestFindByPID(t *testing.T) {
	id := strings.Repeat("a", 64)

	t.Run("not in a container", func(t *testing.T) {
		d := newDockerProvider(t.TempDir() + "/nonexistent.sock")
		d.cgroupForPID = func(int32) string { return "" }
		c, err := d.FindByPID(context.Background(), 42)
		if err != nil {
			t.Fatalf("FindByPID: %v", err)
		}
		if c != nil {
			t.Fatalf("FindByPID = %+v, want nil", c)
		}
	})

	t.Run("matches container in daemon", func(t *testing.T) {
		sock := startDockerServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, testContainerJSON())
		}))
		d := newDockerProvider(sock)
		d.cgroupForPID = func(int32) string { return id }
		c, err := d.FindByPID(context.Background(), 42)
		if err != nil {
			t.Fatalf("FindByPID: %v", err)
		}
		if c == nil || c.Name != "api-1" {
			t.Fatalf("FindByPID = %+v, want api-1", c)
		}
	})

	t.Run("daemon unreachable falls back to ID", func(t *testing.T) {
		d := newDockerProvider(t.TempDir() + "/missing.sock")
		d.cgroupForPID = func(int32) string { return id }
		c, err := d.FindByPID(context.Background(), 42)
		if err != nil {
			t.Fatalf("FindByPID: %v", err)
		}
		if c == nil || c.ID != id {
			t.Fatalf("FindByPID = %+v, want minimal container with ID", c)
		}
	})
}

func TestContainerActions(t *testing.T) {
	var got []string
	sock := startDockerServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		got = append(got, r.Method+" "+r.URL.RequestURI())
		w.WriteHeader(http.StatusNoContent)
	}))
	d := newDockerProvider(sock)
	ctx := context.Background()

	if err := d.Stop(ctx, "abc123", 10*time.Second); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := d.Kill(ctx, "abc123"); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if err := d.Restart(ctx, "abc123", 5*time.Second); err != nil {
		t.Fatalf("Restart: %v", err)
	}

	want := []string{
		"POST /containers/abc123/stop?t=10",
		"POST /containers/abc123/kill?signal=SIGKILL",
		"POST /containers/abc123/restart?t=5",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("requests = %v, want %v", got, want)
	}
}

func TestDockerHTTPError(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error"}
	body := http.NoBody
	resp.Body = body
	err := dockerHTTPError("/containers/x/stop", resp)
	if err == nil || !strings.Contains(err.Error(), "500 Internal Server Error") {
		t.Fatalf("dockerHTTPError = %v", err)
	}
}

var _ ContainerProvider = (*dockerProvider)(nil)
