//go:build linux

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

// Linux port and connection resolution reads the kernel's /proc/net/* tables
// directly, then maps socket inodes back to owning processes by inspecting
// /proc/<pid>/fd symlinks. No external commands are used.

// inodeMapTTL bounds how long the cached inode→pid map is reused. Watch mode
// re-runs resolution on every tick, so a process that starts after the first
// scan is picked up by a later refresh instead of being attributed from a
// map frozen at process start.
const inodeMapTTL = time.Second

type sharedInodeMap struct {
	mu        sync.Mutex
	ttl       time.Duration
	build     func() map[uint64]int32
	inodes    map[uint64]int32
	fetchedAt time.Time
}

func newSharedInodeMap(ttl time.Duration, build func() map[uint64]int32) *sharedInodeMap {
	return &sharedInodeMap{ttl: ttl, build: build}
}

func (s *sharedInodeMap) get() map[uint64]int32 {
	s.mu.Lock()
	if s.inodes != nil && time.Since(s.fetchedAt) < s.ttl {
		m := s.inodes
		s.mu.Unlock()
		return m
	}
	s.mu.Unlock()

	// Build without the lock held: a /proc walk can take hundreds of
	// milliseconds, and serializing every resolver behind it would turn a
	// fresh cache hit into a queue. Concurrent expired callers may both
	// build; the first to publish wins and the loser discards its result.
	m := s.build()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inodes == nil || time.Since(s.fetchedAt) >= s.ttl {
		s.inodes = m
		s.fetchedAt = time.Now()
	}
	return s.inodes
}

var defaultLinuxInodeMap = newSharedInodeMap(inodeMapTTL, socketInodeMap)

// refresh rebuilds the map unconditionally, publishes it, and returns the
// fresh snapshot. ResolvePort uses it after an attribution miss: a socket
// created since the cache was built cannot be owned by any PID recorded in a
// stale map, so consulting the cache again would just return the same miss.
func (s *sharedInodeMap) refresh() map[uint64]int32 {
	m := s.build()
	s.mu.Lock()
	s.inodes = m
	s.fetchedAt = time.Now()
	s.mu.Unlock()
	return m
}

type linuxPortResolver struct {
	inodes *sharedInodeMap
}

type linuxNetworkInspector struct {
	inodes *sharedInodeMap
}

func newPortResolver() PortResolver { return &linuxPortResolver{inodes: defaultLinuxInodeMap} }
func newNetworkInspector() NetworkInspector {
	return &linuxNetworkInspector{inodes: defaultLinuxInodeMap}
}

// socketInodeMap scans /proc/<pid>/fd to build an inode→pid mapping for all
// socket file descriptors on the system. Results are cached for inodeMapTTL.
func socketInodeMap() map[uint64]int32 {
	m := map[uint64]int32{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return m
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") || len(link) <= len("socket:[]") {
				continue
			}
			inode, err := strconv.ParseUint(link[len("socket:["):len(link)-1], 10, 64)
			if err == nil {
				m[inode] = int32(pid)
			}
		}
	}
	return m
}

func (r *linuxPortResolver) inodeMap() map[uint64]int32 {
	return r.inodes.get()
}

func (r *linuxPortResolver) ResolvePort(_ context.Context, port uint16, protocol model.Protocol) ([]model.Listener, error) {
	out := resolvePortRows(port, protocol, r.inodes.get())
	// A row whose PID is missing was created after the cached inode map was
	// built (or is owned by a process whose fds we cannot read). Rebuild once
	// so inspecting a server that just started still attributes its process
	// instead of reporting an ownerless listener.
	if anyUnattributed(out) {
		out = resolvePortRows(port, protocol, r.inodes.refresh())
	}
	return out, nil
}

// resolvePortRows reads the /proc/net tables for one port using the given
// inode map.
func resolvePortRows(port uint16, protocol model.Protocol, inodes map[uint64]int32) []model.Listener {
	proto := protocol.Normalize()
	var out []model.Listener
	if proto == "" || proto == model.ProtocolTCP {
		out = append(out, resolveProcNetPort("/proc/net/tcp", model.ProtocolTCP, port, inodes)...)
		out = append(out, resolveProcNetPort("/proc/net/tcp6", model.ProtocolTCP, port, inodes)...)
	}
	if proto == "" || proto == model.ProtocolUDP {
		out = append(out, resolveProcNetPort("/proc/net/udp", model.ProtocolUDP, port, inodes)...)
		out = append(out, resolveProcNetPort("/proc/net/udp6", model.ProtocolUDP, port, inodes)...)
	}
	return out
}

// anyUnattributed reports whether any listener row has no owning PID.
func anyUnattributed(ls []model.Listener) bool {
	for _, l := range ls {
		if l.PID == 0 {
			return true
		}
	}
	return false
}

func resolveProcNetPort(path string, proto model.Protocol, port uint16, inodes map[uint64]int32) []model.Listener {
	rows, err := parseProcNet(path)
	if err != nil {
		return nil
	}
	var out []model.Listener
	for _, row := range rows {
		if row.localPort != port {
			continue
		}
		if proto == model.ProtocolTCP && row.state != "0A" { // only LISTEN
			continue
		}
		state := "BOUND"
		if proto == model.ProtocolTCP {
			state = "LISTEN"
		}
		out = append(out, model.Listener{
			Protocol: proto,
			Address:  row.localAddr,
			Port:     port,
			State:    state,
			PID:      inodes[row.inode],
		})
	}
	return out
}

func (r *linuxPortResolver) Listeners(_ context.Context) ([]model.Listener, error) {
	inodes := r.inodeMap()
	var out []model.Listener
	for _, spec := range []struct {
		path  string
		proto model.Protocol
	}{
		{"/proc/net/tcp", model.ProtocolTCP},
		{"/proc/net/tcp6", model.ProtocolTCP},
		{"/proc/net/udp", model.ProtocolUDP},
		{"/proc/net/udp6", model.ProtocolUDP},
	} {
		rows, err := parseProcNet(spec.path)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if spec.proto == model.ProtocolTCP && row.state != "0A" {
				continue
			}
			state := "BOUND"
			if spec.proto == model.ProtocolTCP {
				state = "LISTEN"
			}
			out = append(out, model.Listener{
				Protocol: spec.proto,
				Address:  row.localAddr,
				Port:     row.localPort,
				State:    state,
				PID:      inodes[row.inode],
			})
		}
	}
	return out, nil
}

func (n *linuxNetworkInspector) inodeMap() map[uint64]int32 {
	return n.inodes.get()
}

func (n *linuxNetworkInspector) Connections(_ context.Context, pid int32) ([]model.Connection, error) {
	inodes := n.inodeMap()
	owned := map[uint64]bool{}
	for inode, owner := range inodes {
		if owner == pid {
			owned[inode] = true
		}
	}
	if len(owned) == 0 {
		return nil, nil
	}
	var out []model.Connection
	for _, spec := range []struct {
		path  string
		proto model.Protocol
	}{
		{"/proc/net/tcp", model.ProtocolTCP},
		{"/proc/net/tcp6", model.ProtocolTCP},
		{"/proc/net/udp", model.ProtocolUDP},
		{"/proc/net/udp6", model.ProtocolUDP},
	} {
		rows, err := parseProcNet(spec.path)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if !owned[row.inode] {
				continue
			}
			out = append(out, model.Connection{
				PID:        pid,
				Protocol:   spec.proto,
				LocalAddr:  row.localAddr,
				LocalPort:  row.localPort,
				RemoteAddr: row.remoteAddr,
				RemotePort: row.remotePort,
				State:      tcpStateNames[row.state],
			})
		}
	}
	return out, nil
}
