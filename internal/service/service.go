// Package service coordinates core domain use cases for port intelligence,
// decoupled from delivery mechanisms (CLI, APIs, or scripts).
package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/inspector"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// ProgressFunc reports scan progress in a thread-safe manner.
type ProgressFunc func(done, total, found int, elapsed time.Duration)

// PortService defines the business domain facade.
type PortService struct {
	inspector inspector.PortInspector
	platform  *platform.Platform
	actions   *actions.Manager
}

// Option configures a PortService.
type Option func(*PortService)

// WithInspector configures the port inspector implementation.
func WithInspector(insp inspector.PortInspector) Option {
	return func(s *PortService) {
		s.inspector = insp
	}
}

// WithPlatform configures the platform provider.
func WithPlatform(plat *platform.Platform) Option {
	return func(s *PortService) {
		s.platform = plat
	}
}

// WithActions configures the action manager.
func WithActions(act *actions.Manager) Option {
	return func(s *PortService) {
		s.actions = act
	}
}

// New creates a new PortService instance with sensible defaults and functional options.
func New(opts ...Option) *PortService {
	plat := platform.New()
	s := &PortService{
		platform:  plat,
		inspector: inspector.New(plat),
		actions:   actions.NewManager(plat, nil, nil),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// List returns the currently active listeners on the host.
func (s *PortService) List(ctx context.Context, onlyTCP bool) ([]model.PortEntry, error) {
	slog.DebugContext(ctx, "service: listing active listeners", "only_tcp", onlyTCP)
	entries, err := s.inspector.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing active ports: %w", err)
	}
	if !onlyTCP {
		return entries, nil
	}
	var tcpOnly []model.PortEntry
	for _, e := range entries {
		if e.Protocol.Normalize() == model.ProtocolTCP {
			tcpOnly = append(tcpOnly, e)
		}
	}
	return tcpOnly, nil
}

// Inspect returns a report for a single port at the requested depth.
func (s *PortService) Inspect(ctx context.Context, port int32, protocol model.Protocol, depth inspector.Depth) (*model.Report, error) {
	slog.DebugContext(ctx, "service: inspecting port", "port", port, "protocol", protocol, "depth", depth)
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: %d (must be 1-65535)", model.ErrInvalidPort, port)
	}
	report, err := s.inspector.InspectDepth(ctx, port, protocol, depth)
	if err != nil {
		return nil, fmt.Errorf("inspecting port %d: %w", port, err)
	}
	return report, nil
}

// Scan performs parallel port inspection across a collection of ports with live progress reporting.
func (s *PortService) Scan(ctx context.Context, ports []int32, protocol model.Protocol, onProgress ProgressFunc) ([]*model.Report, error) {
	slog.DebugContext(ctx, "service: scanning ports", "count", len(ports), "protocol", protocol)
	if len(ports) == 0 {
		return nil, nil
	}

	start := time.Now()

	// Pre-filter against host's active listeners to avoid redundant OS queries
	var activePorts map[uint16]bool
	if allEntries, err := s.inspector.List(ctx); err == nil {
		activePorts = make(map[uint16]bool, len(allEntries))
		for _, e := range allEntries {
			activePorts[uint16(e.Port)] = true
		}
	}

	type scanResult struct {
		report *model.Report
		err    error
	}

	type portJob struct {
		index int
		port  int32
	}

	results := make([]scanResult, len(ports))
	var toInspect []portJob
	var idleCount int

	for i, p := range ports {
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("%w: %d", model.ErrInvalidPort, p)
		}
		if activePorts != nil && !activePorts[uint16(p)] {
			idleCount++
		} else {
			toInspect = append(toInspect, portJob{index: i, port: p})
		}
	}

	var doneCount atomic.Int64
	var foundCount atomic.Int64
	var progressMu sync.Mutex

	reportProgress := func() {
		if onProgress == nil {
			return
		}
		progressMu.Lock()
		defer progressMu.Unlock()
		onProgress(int(doneCount.Load()), len(ports), int(foundCount.Load()), time.Since(start))
	}

	if idleCount > 0 {
		doneCount.Add(int64(idleCount))
		reportProgress()
	}

	if len(toInspect) > 0 {
		workers := runtime.NumCPU() * 2
		if workers > 16 {
			workers = 16
		}
		if workers > len(toInspect) {
			workers = len(toInspect)
		}
		if workers < 1 {
			workers = 1
		}

		jobs := make(chan portJob, len(toInspect))
		for _, job := range toInspect {
			jobs <- job
		}
		close(jobs)

		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					if ctx.Err() != nil {
						return
					}
					report, err := s.inspector.InspectDepth(ctx, job.port, protocol, inspector.DepthFast)
					results[job.index] = scanResult{report: report, err: err}
					doneCount.Add(1)
					if report != nil && report.Status == "listening" {
						foundCount.Add(1)
					}
					reportProgress()
				}
			}()
		}
		wg.Wait()
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var found []*model.Report
	for _, res := range results {
		if res.err == nil && res.report != nil && res.report.Status == "listening" {
			found = append(found, res.report)
		}
	}

	return found, nil
}

// Kill terminates the process owning the port or specified report.
func (s *PortService) Kill(ctx context.Context, report *model.Report, force bool) error {
	if report == nil {
		return model.ErrPortNotFound
	}
	slog.DebugContext(ctx, "service: killing process on port", "port", report.Port, "force", force)
	return s.actions.Kill(ctx, report, force)
}

// Restart relaunches the process owning the port or specified report.
func (s *PortService) Restart(ctx context.Context, report *model.Report) error {
	if report == nil {
		return model.ErrPortNotFound
	}
	slog.DebugContext(ctx, "service: restarting process on port", "port", report.Port)
	return s.actions.Restart(ctx, report)
}

// Open launches the default browser pointing to the service on the port.
func (s *PortService) Open(ctx context.Context, report *model.Report) error {
	if report == nil {
		return model.ErrPortNotFound
	}
	slog.DebugContext(ctx, "service: opening port in browser", "port", report.Port)
	return s.actions.Open(ctx, report)
}

// Tree resolves the deep report containing process hierarchy for the given port.
func (s *PortService) Tree(ctx context.Context, port int32) (*model.Report, error) {
	report, err := s.Inspect(ctx, port, "", inspector.DepthFull)
	if err != nil {
		return nil, err
	}
	if report.Status != "listening" || report.Process == nil {
		return report, model.ErrPortNotFound
	}
	return report, nil
}

// Connections retrieves the deep report containing network connections for the given port.
func (s *PortService) Connections(ctx context.Context, port int32) (*model.Report, error) {
	report, err := s.Inspect(ctx, port, "", inspector.DepthFull)
	if err != nil {
		return nil, err
	}
	if report.Status != "listening" || report.Process == nil {
		return report, model.ErrPortNotFound
	}
	return report, nil
}

// Find resolves ports listening on the host filtered by process name query or PID.
func (s *PortService) Find(ctx context.Context, query string, pid int) ([]int32, error) {
	slog.DebugContext(ctx, "service: finding ports", "query", query, "pid", pid)
	if pid > 0 {
		entries, err := s.inspector.SearchByPID(ctx, int32(pid))
		if err != nil {
			return nil, fmt.Errorf("finding ports for pid %d: %w", pid, err)
		}
		var ports []int32
		for _, e := range entries {
			ports = append(ports, e.Port)
		}
		return ports, nil
	}
	if query != "" {
		entries, err := s.inspector.SearchByName(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("finding ports matching %q: %w", query, err)
		}
		var ports []int32
		for _, e := range entries {
			ports = append(ports, e.Port)
		}
		return ports, nil
	}
	return nil, fmt.Errorf("%w: query or pid required", model.ErrInvalidArguments)
}

// NextAvailable finds the lowest unused, bindable port starting from startPort.
func (s *PortService) NextAvailable(ctx context.Context, startPort int32) (int32, error) {
	if startPort < 1 || startPort > 65535 {
		startPort = 3000
	}
	slog.DebugContext(ctx, "service: searching next available port", "start_port", startPort)

	for p := startPort; p <= 65535; p++ {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		// Probe TCP bindability
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(int(p)))
		if err == nil {
			_ = ln.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("%w: no available ports found above %d", model.ErrPortNotFound, startPort)
}
