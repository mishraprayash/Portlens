// Package platform defines PortLens's operating-system abstraction layer.
//
// All OS interaction flows through the interfaces declared here. Concrete
// implementations live in build-tagged files (darwin_*.go, linux_*.go) so that
// Windows support can be added later without touching the rest of the codebase.
//
// Platform-specific shell commands (lsof, netstat, ss, ps, pbcopy, ...) are
// permitted as a fallback but are strictly confined to the implementation
// files; no command is ever shelled out to from higher layers.
package platform

import (
	"context"
	"time"

	"github.com/portlens/portlens/internal/model"
)

// Signal is a portable process signal selector.
type Signal int

const (
	// SignalTerm is SIGTERM (graceful termination).
	SignalTerm Signal = iota
	// SignalKill is SIGKILL (forced termination).
	SignalKill
	// SignalInterrupt is SIGINT.
	SignalInterrupt
)

// PortResolver maps ports to the sockets (and owning processes) that use them.
type PortResolver interface {
	// Listeners returns every listening socket currently on the host.
	Listeners(ctx context.Context) ([]model.Listener, error)

	// ResolvePort returns all listeners matching the given port. An empty
	// protocol string matches any protocol family.
	ResolvePort(ctx context.Context, port uint16, protocol model.Protocol) ([]model.Listener, error)
}

// ProcessInspector inspects individual OS processes.
type ProcessInspector interface {
	// Info returns basic metadata for a PID, or ErrProcessNotFound.
	Info(ctx context.Context, pid int32) (*model.ProcessInfo, error)

	// Children returns the direct children of a PID.
	Children(ctx context.Context, pid int32) ([]*model.ProcessInfo, error)

	// Exists reports whether the process is still alive.
	Exists(ctx context.Context, pid int32) bool
}

// ProcessTreeProvider builds process hierarchies.
type ProcessTreeProvider interface {
	// Ancestors returns the chain of parents from the PID up to the root
	// (init/launchd), oldest last.
	Ancestors(ctx context.Context, pid int32) ([]*model.ProcessInfo, error)

	// Descendants returns the full descendant tree of a PID.
	Descendants(ctx context.Context, pid int32) (*model.ProcessTree, error)
}

// NetworkInspector inspects network connections.
type NetworkInspector interface {
	// Connections returns all active connections owned by a PID.
	Connections(ctx context.Context, pid int32) ([]model.Connection, error)

	// ListenersForPID returns the sockets a PID is listening on.
	ListenersForPID(ctx context.Context, pid int32) ([]model.Listener, error)
}

// ClipboardProvider copies text to the system clipboard.
type ClipboardProvider interface {
	Copy(ctx context.Context, text string) error
}

// ProcessController sends signals to processes.
type ProcessController interface {
	// Signal sends the selected signal to a PID.
	Signal(ctx context.Context, pid int32, sig Signal) error

	// IsAlive reports whether a PID still exists.
	IsAlive(ctx context.Context, pid int32) bool
}

// ContainerProvider identifies and controls containers. Detection is
// best-effort: when no container runtime is reachable, the Find* methods
// return nil with no error so callers can degrade gracefully. A nil result
// never means "this is not a container"; it means "not determined".
type ContainerProvider interface {
	// FindByPort returns the container publishing a host port, or nil when no
	// container publishes it.
	FindByPort(ctx context.Context, port uint16, protocol model.Protocol) (*model.Container, error)

	// FindByPorts returns the containers publishing any of the given host
	// ports, keyed by port. It performs a single runtime query.
	FindByPorts(ctx context.Context, ports []uint16, protocol model.Protocol) (map[uint16]*model.Container, error)

	// FindByPID returns the container a host process runs inside, or nil when
	// the process is not inside a container (or it cannot be determined).
	FindByPID(ctx context.Context, pid int32) (*model.Container, error)

	// Stop gracefully stops a container (SIGTERM to its init process), waiting
	// up to timeout seconds for it to exit.
	Stop(ctx context.Context, containerID string, timeout time.Duration) error

	// Kill force-stops a container (SIGKILL to its init process).
	Kill(ctx context.Context, containerID string) error

	// Restart restarts a container, waiting up to timeout seconds.
	Restart(ctx context.Context, containerID string, timeout time.Duration) error
}

// Platform bundles the OS-specific providers into a single handle.
type Platform struct {
	Ports      PortResolver
	Processes  ProcessInspector
	Network    NetworkInspector
	Tree       ProcessTreeProvider
	Clipboard  ClipboardProvider
	Controller ProcessController
	Containers ContainerProvider
}
