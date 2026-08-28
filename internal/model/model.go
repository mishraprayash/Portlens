// Package model defines the core data types shared across PortLens.
//
// These types are intentionally OS-independent. Platform-specific details are
// isolated behind the interfaces in the platform package, and only these
// normalized structures are surfaced to the rendering and JSON layers.
package model

import (
	"fmt"
	"strconv"
	"time"
)

// Protocol identifies a transport protocol.
type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolTCP6 Protocol = "tcp6"
	ProtocolUDP6 Protocol = "udp6"
)

// Normalize collapses IPv4/IPv6 variants into the canonical family name so
// that callers can compare protocols without caring about address family.
func (p Protocol) Normalize() Protocol {
	switch p {
	case ProtocolTCP, ProtocolTCP6:
		return ProtocolTCP
	case ProtocolUDP, ProtocolUDP6:
		return ProtocolUDP
	default:
		return p
	}
}

// Listener describes a single listening socket on the host.
type Listener struct {
	Protocol Protocol `json:"protocol"`
	Address  string   `json:"address"` // bind address, e.g. "127.0.0.1" or "0.0.0.0"
	Port     uint16   `json:"port"`
	State    string   `json:"state"` // e.g. "LISTEN"
	PID      int32    `json:"pid,omitempty"`
	Process  string   `json:"process,omitempty"` // process name, if known
}

// Key returns a stable key for a listener (family + port).
func (l Listener) Key() string {
	return string(l.Protocol.Normalize()) + ":" + itoa(int(l.Port))
}

// Connection describes a single active network connection owned by a process.
type Connection struct {
	PID        int32    `json:"pid,omitempty"`
	Protocol   Protocol `json:"protocol"`
	LocalAddr  string   `json:"local_address"`
	LocalPort  uint16   `json:"local_port"`
	RemoteAddr string   `json:"remote_address,omitempty"`
	RemotePort uint16   `json:"remote_port,omitempty"`
	State      string   `json:"state"` // ESTABLISHED, TIME_WAIT, ...
}

// NetworkSummary summarizes a process's active connections.
type NetworkSummary struct {
	Total       int            `json:"total"`
	ByState     map[string]int `json:"by_state"`
	LocalOnly   bool           `json:"local_only"`   // every connection is loopback
	RemoteCount int            `json:"remote_count"` // connections with non-loopback remote peers
}

// ProcessInfo describes a single OS process.
type ProcessInfo struct {
	PID         int32     `json:"pid"`
	PPID        int32     `json:"ppid,omitempty"`
	Name        string    `json:"name,omitempty"`
	Exe         string    `json:"exe,omitempty"`
	Command     string    `json:"command,omitempty"` // human-friendly command line
	Cmdline     []string  `json:"cmdline,omitempty"`
	CWD         string    `json:"cwd,omitempty"`
	User        string    `json:"user,omitempty"`
	StartTime   time.Time `json:"started_at,omitempty"`
	CPUPercent  float64   `json:"cpu_percent,omitempty"`
	MemoryBytes uint64    `json:"memory_bytes,omitempty"`
	Terminal    string    `json:"terminal,omitempty"`
	IsZombie    bool      `json:"is_zombie,omitempty"`
	IsTarget    bool      `json:"is_target,omitempty"` // true when this process owns the target port
}

// Runtime returns a human-readable running duration, or "" if unknown.
func (p ProcessInfo) Runtime(now time.Time) string {
	if p.StartTime.IsZero() {
		return ""
	}
	d := now.Sub(p.StartTime)
	if d < 0 {
		d = 0
	}
	return FormatDuration(d)
}

// Container describes a container that owns or publishes a port. Every field
// is a fact obtained from the container runtime; nothing here is guessed.
type Container struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`   // e.g. "api-1"
	Image          string `json:"image,omitempty"`  // e.g. "nginx:alpine"
	Status         string `json:"status,omitempty"` // e.g. "running", "exited"
	ComposeProject string `json:"compose_project,omitempty"`
	ComposeService string `json:"compose_service,omitempty"`
}

// ProcessTree is a node in a process hierarchy.
type ProcessTree struct {
	Process  ProcessInfo    `json:"process"`
	Children []*ProcessTree `json:"children,omitempty"`
}

// ProjectInfo describes metadata inferred about a process's home project.
type ProjectInfo struct {
	Name           string `json:"name,omitempty"`
	Directory      string `json:"directory,omitempty"`
	GitRepo        string `json:"git_repo,omitempty"`
	GitBranch      string `json:"git_branch,omitempty"`
	Runtime        string `json:"runtime,omitempty"`         // node, python, go, java, ...
	Framework      string `json:"framework,omitempty"`       // nestjs, express, next.js, ...
	PackageManager string `json:"package_manager,omitempty"` // pnpm, npm, yarn, pip, cargo, ...
	Language       string `json:"language,omitempty"`        // best-guess primary language
	Detected       bool   `json:"detected"`                  // true when project metadata was found
}

// RiskLevel is a cautious classification of a finding.
type RiskLevel string

const (
	RiskLow       RiskLevel = "LOW RISK"
	RiskWarning   RiskLevel = "WARNING"
	RiskDangerous RiskLevel = "POTENTIALLY DANGEROUS"
)

// Finding is a single risk/exposure observation. PortLens deliberately avoids
// claiming anything is definitively safe; findings are phrased cautiously.
type Finding struct {
	Level  RiskLevel `json:"level"`
	Reason string    `json:"reason"`
}

// Exposure is the overall exposure verdict for a listener.
type Exposure struct {
	BoundLocalhost  bool      `json:"bound_localhost"`
	BoundWildcard   bool      `json:"bound_wildcard"`
	PublicInterface bool      `json:"public_interface"`
	Findings        []Finding `json:"findings"`
	Worst           RiskLevel `json:"worst_level"`
}

// Origin classifies whether the owning process is bundled with the operating
// system ("system") or was installed/launched by the user ("third-party"). It
// is a heuristic, so the empty value (unknown) is also valid.
type Origin string

const (
	OriginSystem  Origin = "system" // part of the operating system (e.g. macOS/Apple)
	OriginUser    Origin = "user"   // installed or launched by the user (third-party)
	OriginUnknown Origin = "unknown"
)

// Report is the full inspection result for a single port. It is the source of
// truth for both the human-facing terminal UI and the machine-readable JSON.
type Report struct {
	Port      int32          `json:"port"`
	Protocol  Protocol       `json:"protocol"`
	Status    string         `json:"status"` // listening, not_listening, unknown
	Address   string         `json:"address"`
	Service   string         `json:"service,omitempty"` // well-known service name for the port
	Process   *ProcessInfo   `json:"process,omitempty"`
	Origin    Origin         `json:"origin,omitempty"` // system | user (heuristic)
	Container *Container     `json:"container,omitempty"`
	Ancestors []*ProcessInfo `json:"ancestors,omitempty"`
	Children  []*ProcessInfo `json:"children,omitempty"`
	// Descendants holds the full descendant tree. It is used only for the
	// human-facing tree rendering and is omitted from JSON output (the flat
	// Children slice above is the machine-readable form).
	Descendants    *ProcessTree `json:"-"`
	Project        *ProjectInfo `json:"project,omitempty"`
	Network        *NetworkInfo `json:"network,omitempty"`
	Exposure       *Exposure    `json:"exposure,omitempty"`
	HTTPProbe      *HTTPProbe   `json:"http_probe,omitempty"`
	Interpretation string       `json:"interpretation,omitempty"`

	// Facts records concrete observations. Inference records interpretations
	// that PortLens guessed. They are deliberately separated so callers can
	// trust facts and treat inferences with appropriate skepticism.
	Facts      []string `json:"facts,omitempty"`
	Inferences []string `json:"inferences,omitempty"`
}

// HTTPProbe contains lightweight HTTP inspection results when an endpoint responds to HTTP GET.
type HTTPProbe struct {
	Status     string        `json:"status,omitempty"`      // e.g. "200 OK"
	StatusCode int           `json:"status_code,omitempty"` // e.g. 200
	Title      string        `json:"title,omitempty"`       // extracted HTML <title>
	Server     string        `json:"server,omitempty"`      // Server response header
	Latency    time.Duration `json:"latency,omitempty"`     // round-trip time
}

// NetworkInfo describes the network footprint of the owning process.
type NetworkInfo struct {
	Listeners   []Listener     `json:"listeners"`
	Connections []Connection   `json:"connections,omitempty"`
	Summary     NetworkSummary `json:"summary"`
}

// PortEntry is a single row in the "interesting ports" listing.
type PortEntry struct {
	Port      int32      `json:"port"`
	Protocol  Protocol   `json:"protocol"`
	Process   string     `json:"process,omitempty"`
	PID       int32      `json:"pid,omitempty"`
	Service   string     `json:"service,omitempty"` // well-known service name for the port
	Container *Container `json:"container,omitempty"`
	Project   string     `json:"project,omitempty"`
	Runtime   string     `json:"runtime,omitempty"`
	Address   string     `json:"address"`
	Status    string     `json:"status"`
	Origin    Origin     `json:"origin,omitempty"` // system | user (heuristic)
}

// HistoryEntry is a single observation of a port over time.
type HistoryEntry struct {
	Port       int32     `json:"port"`
	ObservedAt time.Time `json:"observed_at"`
	PID        int32     `json:"pid,omitempty"`
	Process    string    `json:"process,omitempty"`
	Project    string    `json:"project,omitempty"`
	Command    string    `json:"command,omitempty"`
	Address    string    `json:"address,omitempty"`
	Status     string    `json:"status"` // started, exited, seen
}

// FormatDuration renders a duration in a compact human-friendly form.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return itoa(h) + "h " + itoa(m) + "m"
	}
	if m == 0 {
		return d.Round(time.Second).String()
	}
	return itoa(m) + "m"
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// FormatBytes renders a byte count in human-friendly units (B, KB, MB, GB).
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return strconv.FormatUint(b, 10) + " B"
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	val := float64(b) / float64(div)
	if val >= 10 || exp == 0 {
		return fmt.Sprintf("%.0f %s", val, units[exp])
	}
	return fmt.Sprintf("%.1f %s", val, units[exp])
}
