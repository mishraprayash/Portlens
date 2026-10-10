// Package service coordinates core domain use cases for port intelligence,
// decoupled from delivery mechanisms (CLI, APIs, or scripts).
package service

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mishraprayash/Portlens/internal/inspector"
	"github.com/mishraprayash/Portlens/internal/model"
)

// ProgressFunc reports scan progress in a thread-safe manner.
type ProgressFunc func(done, total, found int, elapsed time.Duration)

// PortService defines the business domain facade.
type PortService struct {
	inspector inspector.PortInspector
}

// Option configures a PortService.
type Option func(*PortService)

// WithInspector configures the port inspector implementation.
func WithInspector(insp inspector.PortInspector) Option {
	return func(s *PortService) {
		s.inspector = insp
	}
}

// New creates a new PortService instance with sensible defaults and functional options.
func New(opts ...Option) *PortService {
	s := &PortService{}
	for _, opt := range opts {
		opt(s)
	}
	if s.inspector == nil {
		s.inspector = inspector.New(nil)
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

// ScanResult holds the outcome of a parallel scan: ports confirmed in use,
// plus any per-port inspection failures — without them a port whose
// inspection errored would be indistinguishable from an idle port.
type ScanResult struct {
	Reports  []*model.Report
	Failed   int   // ports whose inspection errored
	FirstErr error // first inspection error, for reporting
}

// Scan performs parallel port inspection across a collection of ports with
// live progress reporting. A non-nil error means the scan could not run at
// all (invalid port, canceled context); per-port failures are reported via
// ScanResult instead of failing the scan.
func (s *PortService) Scan(ctx context.Context, ports []int32, protocol model.Protocol, onProgress ProgressFunc) (ScanResult, error) {
	slog.DebugContext(ctx, "service: scanning ports", "count", len(ports), "protocol", protocol)
	if len(ports) == 0 {
		return ScanResult{}, nil
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
			return ScanResult{}, fmt.Errorf("%w: %d", model.ErrInvalidPort, p)
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
		return ScanResult{}, ctx.Err()
	}

	var res ScanResult
	for _, r := range results {
		if r.err != nil {
			res.Failed++
			if res.FirstErr == nil {
				res.FirstErr = r.err
			}
			continue
		}
		if r.report != nil && r.report.Status == "listening" {
			res.Reports = append(res.Reports, r.report)
		}
	}

	return res, nil
}
