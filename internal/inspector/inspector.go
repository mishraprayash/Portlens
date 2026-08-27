// Package inspector orchestrates the platform providers into a complete
// inspection report for a port. It is the seam between the OS abstraction
// layer, the detection heuristics, and the renderers/actions.
package inspector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// ErrPortNotFound is returned when nothing is listening on the requested port.
var ErrPortNotFound = fmt.Errorf("no process is listening on this port")

// Inspector performs port and process inspection.
type Inspector struct {
	Platform *platform.Platform
	Projects detect.ProjectDetector
	Now      func() time.Time
}

// New builds an Inspector with the given platform and default detectors.
func New(p *platform.Platform) *Inspector {
	return &Inspector{
		Platform: p,
		Projects: detect.NewProjectDetector(),
		Now:      time.Now,
	}
}

// Inspect produces a full report for a single port.
func (i *Inspector) Inspect(ctx context.Context, port int32, protocol model.Protocol) (*model.Report, error) {
	report := &model.Report{Port: port, Protocol: protocol, Status: "not_listening"}

	listeners, err := i.Platform.Ports.ResolvePort(ctx, uint16(port), protocol)
	if err != nil {
		return report, err
	}
	if len(listeners) == 0 {
		return report, ErrPortNotFound
	}

	primary := choosePrimary(listeners)
	report.Status = "listening"
	report.Protocol = primary.Protocol
	report.Address = primary.Address

	pid := primary.PID
	if pid <= 0 {
		// Socket exists but we could not attribute it to a process (privileged
		// process, or permission limits). Report what we know.
		report.Exposure = assessExposure(listeners)
		report.Facts = append(report.Facts,
			fmt.Sprintf("Port %d is bound to %s:%d", port, displayAddr(primary.Address), port))
		report.Inferences = append(report.Inferences,
			"Owner could not be determined (may require elevated privileges)")
		return report, nil
	}

	proc, err := i.Platform.Processes.Info(ctx, pid)
	if err != nil {
		report.Status = "listening"
		report.Facts = append(report.Facts, fmt.Sprintf("Port %d was bound to process %d which has since exited", port, pid))
		return report, nil
	}
	proc.IsTarget = true
	report.Process = proc

	// Build process hierarchy.
	if ancestors, err := i.Platform.Tree.Ancestors(ctx, pid); err == nil {
		report.Ancestors = ancestors
	}
	if children, err := i.Platform.Processes.Children(ctx, pid); err == nil {
		report.Children = children
	}
	if tree, err := i.Platform.Tree.Descendants(ctx, pid); err == nil {
		report.Descendants = tree
	}

	// Project detection from the process working directory.
	report.Project = i.Projects.Detect(ctx, proc.CWD)

	// Runtime / framework detection.
	runtime := detect.DetectRuntime(proc)
	if runtime == "" && report.Project != nil {
		runtime = report.Project.Runtime
	}
	if report.Project != nil && report.Project.Runtime == "" {
		report.Project.Runtime = runtime
	}
	framework := detect.DetectFramework(proc, report.Project)
	if report.Project != nil && framework != "" {
		report.Project.Framework = framework
	}

	// Network footprint.
	report.Network = i.networkInfo(ctx, pid, listeners)
	report.Exposure = assessExposure(listeners)

	// Interpretation.
	i.interpret(report)

	return report, nil
}

// choosePrimary prefers TCP over UDP and picks the first entry otherwise.
func choosePrimary(listeners []model.Listener) model.Listener {
	first := listeners[0]
	for _, l := range listeners {
		if l.Protocol.Normalize() == model.ProtocolTCP {
			return l
		}
	}
	return first
}

func (i *Inspector) networkInfo(ctx context.Context, pid int32, listeners []model.Listener) *model.NetworkInfo {
	info := &model.NetworkInfo{Listeners: listeners}
	conns, err := i.Platform.Network.Connections(ctx, pid)
	if err == nil {
		info.Connections = conns
	}
	info.Summary = summarize(conns)
	return info
}

func summarize(conns []model.Connection) model.NetworkSummary {
	s := model.NetworkSummary{ByState: map[string]int{}}
	s.Total = len(conns)
	localOnly := true
	for _, c := range conns {
		state := c.State
		if state == "" {
			state = "UNKNOWN"
		}
		s.ByState[state]++
		if !isLoopback(c.RemoteAddr) && c.RemoteAddr != "" {
			localOnly = false
			s.RemoteCount++
		}
	}
	s.LocalOnly = localOnly
	return s
}

// List returns a summary of all listening ports for the no-argument command.
func (i *Inspector) List(ctx context.Context) ([]model.PortEntry, error) {
	listeners, err := i.Platform.Ports.Listeners(ctx)
	if err != nil {
		return nil, err
	}
	return i.buildEntries(ctx, listeners, i.processInfos(ctx, listeners), nil), nil
}

func isLoopback(addr string) bool {
	if addr == "127.0.0.1" || addr == "::1" || addr == "localhost" {
		return true
	}
	return strings.HasPrefix(addr, "127.")
}

func displayAddr(addr string) string {
	if addr == "" {
		return "0.0.0.0"
	}
	return addr
}
