package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/portlens/portlens/internal/model"
)

// dockerSocketTimeout bounds a single daemon round-trip. Detection must never
// make a port inspection feel hung when the daemon is unresponsive.
const dockerSocketTimeout = 2 * time.Second

// dockerActionTimeout bounds container stop/restart, which legitimately wait
// for the container to shut down.
const dockerActionTimeout = 30 * time.Second

// dockerProvider talks to the local Docker daemon over its unix socket using
// the standard HTTP API (docker API v1.24+, compatible with current engines).
// No external command is ever shelled out to.
type dockerProvider struct {
	base   string // e.g. "http://docker"
	socket string
	client *http.Client
	// cgroupForPID resolves the cgroup-derived container ID for a PID. It is a
	// field so tests can inject a deterministic value instead of reading the
	// host's /proc.
	cgroupForPID func(pid int32) string
}

// newDockerProvider builds a provider dialing the daemon over a unix socket.
func newDockerProvider(socket string) *dockerProvider {
	return &dockerProvider{
		base:         "http://docker",
		socket:       socket,
		cgroupForPID: containerIDForPID,
		client: &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: dockerSocketTimeout}).DialContext(ctx, "unix", socket)
			},
		}},
	}
}

// newContainerProvider returns a ContainerProvider for the local Docker daemon,
// or nil when no docker socket is reachable so inspection degrades gracefully.
func newContainerProvider() ContainerProvider {
	for _, path := range dockerSocketPaths() {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return newDockerProvider(path)
		}
	}
	return nil
}

// dockerSocketPaths returns the candidate docker socket paths in order of
// preference, honoring DOCKER_HOST when it points at a unix socket.
func dockerSocketPaths() []string {
	var out []string
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		if strings.HasPrefix(h, "unix://") {
			out = append(out, strings.TrimPrefix(h, "unix://"))
		}
	}
	out = append(out, "/var/run/docker.sock")
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".docker", "run", "docker.sock"))
	}
	return out
}

// containerSummary is a single item in the docker /containers/json response.
type containerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
	Ports  []containerPort   `json:"Ports"`
}

type containerPort struct {
	IP          string `json:"IP"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort"`
	Type        string `json:"Type"`
}

const (
	composeProjectLabel = "com.docker.compose.project"
	composeServiceLabel = "com.docker.compose.service"
)

func containerFromSummary(s containerSummary) *model.Container {
	c := &model.Container{
		ID:     s.ID,
		Image:  s.Image,
		Status: s.State,
	}
	if len(s.Names) > 0 {
		c.Name = strings.TrimPrefix(s.Names[0], "/")
	}
	if s.Labels != nil {
		c.ComposeProject = s.Labels[composeProjectLabel]
		c.ComposeService = s.Labels[composeServiceLabel]
	}
	return c
}

func (d *dockerProvider) FindByPorts(ctx context.Context, ports []uint16, protocol model.Protocol) (map[uint16]*model.Container, error) {
	out := map[uint16]*model.Container{}
	if len(ports) == 0 {
		return out, nil
	}
	summaries, err := d.list(ctx)
	if err != nil {
		return nil, err
	}
	want := make(map[uint16]bool, len(ports))
	for _, p := range ports {
		want[p] = true
	}
	proto := protocol.Normalize()
	for _, s := range summaries {
		c := containerFromSummary(s)
		for _, pm := range s.Ports {
			if !want[pm.PublicPort] {
				continue
			}
			if proto != "" && pm.Type != "" && pm.Type != string(proto) {
				continue
			}
			out[pm.PublicPort] = c
		}
	}
	return out, nil
}

func (d *dockerProvider) FindByPort(ctx context.Context, port uint16, protocol model.Protocol) (*model.Container, error) {
	m, err := d.FindByPorts(ctx, []uint16{port}, protocol)
	if err != nil {
		return nil, err
	}
	return m[port], nil
}

// FindByPID resolves the container a host process runs inside. On Linux this
// uses the process cgroup (a kernel fact that needs no daemon); on other
// platforms the host process is not inside the container runtime's cgroup, so
// this returns nil and callers fall back to a port-based lookup.
func (d *dockerProvider) FindByPID(ctx context.Context, pid int32) (*model.Container, error) {
	id := d.cgroupForPID(pid)
	if id == "" {
		return nil, nil
	}
	summaries, err := d.list(ctx)
	if err != nil {
		// The daemon is unreachable; still report the cgroup-derived ID.
		return &model.Container{ID: id}, nil
	}
	for _, s := range summaries {
		if s.ID == id || strings.HasPrefix(s.ID, id) {
			c := containerFromSummary(s)
			if c.ID == "" {
				c.ID = id
			}
			return c, nil
		}
	}
	return &model.Container{ID: id}, nil
}

func (d *dockerProvider) Stop(ctx context.Context, containerID string, timeout time.Duration) error {
	return d.post(ctx, pathForContainer(containerID)+"/stop", dockerActionTimeout, "t="+fmt.Sprintf("%d", int(timeout.Seconds())))
}

func (d *dockerProvider) Kill(ctx context.Context, containerID string) error {
	return d.post(ctx, pathForContainer(containerID)+"/kill", dockerActionTimeout, "signal=SIGKILL")
}

func (d *dockerProvider) Restart(ctx context.Context, containerID string, timeout time.Duration) error {
	return d.post(ctx, pathForContainer(containerID)+"/restart", dockerActionTimeout, "t="+fmt.Sprintf("%d", int(timeout.Seconds())))
}

func pathForContainer(containerID string) string {
	return "/containers/" + containerID
}

func (d *dockerProvider) list(ctx context.Context) ([]containerSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, dockerSocketTimeout)
	defer cancel()
	var out []containerSummary
	if err := d.get(ctx, "/containers/json", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *dockerProvider) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("docker daemon unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return dockerHTTPError(path, resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *dockerProvider) post(ctx context.Context, path string, timeout time.Duration, query string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	full := d.base + path
	if query != "" {
		full += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("docker daemon unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return dockerHTTPError(path, resp)
	}
	return nil
}

func dockerHTTPError(path string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("docker daemon %s failed: %s", path, msg)
}
