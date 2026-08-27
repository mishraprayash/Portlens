//go:build linux

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/portlens/portlens/internal/model"
)

// Linux port and connection resolution reads the kernel's /proc/net/* tables
// directly, then maps socket inodes back to owning processes by inspecting
// /proc/<pid>/fd symlinks. No external commands are used.

type linuxPortResolver struct {
	inodeOnce sync.Once
	inodes    map[uint64]int32
}

type linuxNetworkInspector struct {
	inodeOnce sync.Once
	inodes    map[uint64]int32
}

func newPortResolver() PortResolver         { return &linuxPortResolver{} }
func newNetworkInspector() NetworkInspector { return &linuxNetworkInspector{} }

// socketInodeMap scans /proc/<pid>/fd to build an inode→pid mapping for all
// socket file descriptors on the system. It is built once per invocation.
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
			if err != nil || !strings.HasPrefix(link, "socket:[") {
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
	r.inodeOnce.Do(func() { r.inodes = socketInodeMap() })
	return r.inodes
}

func (r *linuxPortResolver) ResolvePort(_ context.Context, port uint16, protocol model.Protocol) ([]model.Listener, error) {
	proto := protocol.Normalize()
	inodes := r.inodeMap()
	var out []model.Listener
	if proto == "" || proto == model.ProtocolTCP {
		out = append(out, resolveProcNetPort("/proc/net/tcp", model.ProtocolTCP, port, inodes)...)
		out = append(out, resolveProcNetPort("/proc/net/tcp6", model.ProtocolTCP, port, inodes)...)
	}
	if proto == "" || proto == model.ProtocolUDP {
		out = append(out, resolveProcNetPort("/proc/net/udp", model.ProtocolUDP, port, inodes)...)
		out = append(out, resolveProcNetPort("/proc/net/udp6", model.ProtocolUDP, port, inodes)...)
	}
	return out, nil
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
	n.inodeOnce.Do(func() { n.inodes = socketInodeMap() })
	return n.inodes
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

func (n *linuxNetworkInspector) ListenersForPID(_ context.Context, pid int32) ([]model.Listener, error) {
	inodes := n.inodeMap()
	owned := map[uint64]bool{}
	for inode, owner := range inodes {
		if owner == pid {
			owned[inode] = true
		}
	}
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
			if !owned[row.inode] {
				continue
			}
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
				PID:      pid,
			})
		}
	}
	return out, nil
}
