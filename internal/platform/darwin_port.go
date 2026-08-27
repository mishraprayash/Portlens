//go:build darwin

package platform

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/portlens/portlens/internal/model"
)

// darwinPortResolver and darwinNetworkInspector resolve ports and connections
// by shelling out to lsof. lsof is isolated here, behind the PortResolver and
// NetworkInspector interfaces; no other layer invokes it.
type darwinPortResolver struct{}
type darwinNetworkInspector struct{}

func newPortResolver() PortResolver         { return darwinPortResolver{} }
func newNetworkInspector() NetworkInspector { return darwinNetworkInspector{} }

// runLsof executes lsof with the given arguments and returns stdout.
func runLsof(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "lsof", args...)
	out, err := cmd.Output()
	if err != nil {
		// lsof exits 1 when nothing matches; that is a valid empty result.
		if _, ok := err.(*exec.ExitError); ok && len(out) == 0 {
			return "", nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	return string(out), nil
}

func (r darwinPortResolver) ResolvePort(ctx context.Context, port uint16, protocol model.Protocol) ([]model.Listener, error) {
	proto := protocol.Normalize()
	var out []model.Listener
	var err error
	if proto == "" || proto == model.ProtocolTCP {
		var tcp []model.Listener
		tcp, err = tcpListeners(ctx, port)
		out = append(out, tcp...)
	}
	if err == nil && (proto == "" || proto == model.ProtocolUDP) {
		var udp []model.Listener
		udp, err = udpSockets(ctx, port)
		out = append(out, udp...)
	}
	return out, err
}

func (r darwinPortResolver) Listeners(ctx context.Context) ([]model.Listener, error) {
	data, err := runLsof(ctx, "-nP", "-iTCP", "-sTCP:LISTEN", "-Fpctn")
	if err != nil {
		return nil, err
	}
	var out []model.Listener
	for _, rec := range parseLsofFields(data) {
		if addr, port, ok := parseSockName(rec.name); ok {
			out = append(out, listenerFromRecord(rec, addr, port, model.ProtocolTCP))
		}
	}
	udp, err := runLsof(ctx, "-nP", "-iUDP", "-Fpctn")
	if err != nil {
		return nil, err
	}
	for _, rec := range parseLsofFields(udp) {
		if addr, port, ok := parseSockName(rec.name); ok {
			out = append(out, listenerFromRecord(rec, addr, port, model.ProtocolUDP))
		}
	}
	return out, nil
}

func tcpListeners(ctx context.Context, port uint16) ([]model.Listener, error) {
	data, err := runLsof(ctx, "-nP", "-iTCP:"+strconv.Itoa(int(port)), "-sTCP:LISTEN", "-Fpctn")
	if err != nil {
		return nil, err
	}
	var out []model.Listener
	for _, rec := range parseLsofFields(data) {
		if addr, p, ok := parseSockName(rec.name); ok {
			out = append(out, listenerFromRecord(rec, addr, p, model.ProtocolTCP))
		}
	}
	return out, nil
}

func udpSockets(ctx context.Context, port uint16) ([]model.Listener, error) {
	data, err := runLsof(ctx, "-nP", "-iUDP:"+strconv.Itoa(int(port)), "-Fpctn")
	if err != nil {
		return nil, err
	}
	var out []model.Listener
	for _, rec := range parseLsofFields(data) {
		if addr, p, ok := parseSockName(rec.name); ok {
			out = append(out, listenerFromRecord(rec, addr, p, model.ProtocolUDP))
		}
	}
	return out, nil
}

func listenerFromRecord(rec lsofField, addr string, port uint16, proto model.Protocol) model.Listener {
	state := rec.state
	if state == "" {
		if proto == model.ProtocolUDP {
			state = "BOUND"
		} else {
			state = "LISTEN"
		}
	}
	return model.Listener{
		Protocol: proto,
		Address:  normalizeAddr(addr, rec.family),
		Port:     port,
		State:    state,
		PID:      rec.pid,
		Process:  rec.command,
	}
}

func (d darwinNetworkInspector) Connections(ctx context.Context, pid int32) ([]model.Connection, error) {
	var out []model.Connection
	for _, proto := range []model.Protocol{model.ProtocolTCP, model.ProtocolUDP} {
		arg := "-iTCP"
		if proto == model.ProtocolUDP {
			arg = "-iUDP"
		}
		data, err := runLsof(ctx, "-nP", "-a", "-p", strconv.Itoa(int(pid)), arg, "-FpctnT")
		if err != nil {
			return nil, err
		}
		for _, rec := range parseLsofFields(data) {
			if !strings.Contains(rec.name, "->") {
				continue
			}
			parts := strings.SplitN(rec.name, "->", 2)
			localAddr, localPort, ok1 := parseSockName(parts[0])
			remoteAddr, remotePort, ok2 := parseSockName(parts[1])
			if !ok1 || !ok2 {
				continue
			}
			state := rec.state
			if state == "" {
				state = "ESTABLISHED"
			}
			out = append(out, model.Connection{
				PID:        pid,
				Protocol:   proto,
				LocalAddr:  normalizeAddr(localAddr, rec.family),
				LocalPort:  localPort,
				RemoteAddr: normalizeAddr(remoteAddr, rec.family),
				RemotePort: remotePort,
				State:      state,
			})
		}
	}
	return out, nil
}

func (d darwinNetworkInspector) ListenersForPID(ctx context.Context, pid int32) ([]model.Listener, error) {
	data, err := runLsof(ctx, "-nP", "-a", "-p", strconv.Itoa(int(pid)), "-iTCP", "-sTCP:LISTEN", "-Fpctn")
	if err != nil {
		return nil, err
	}
	var out []model.Listener
	for _, rec := range parseLsofFields(data) {
		if addr, port, ok := parseSockName(rec.name); ok {
			out = append(out, listenerFromRecord(rec, addr, port, model.ProtocolTCP))
		}
	}
	udp, err := runLsof(ctx, "-nP", "-a", "-p", strconv.Itoa(int(pid)), "-iUDP", "-Fpctn")
	if err != nil {
		return nil, err
	}
	for _, rec := range parseLsofFields(udp) {
		if addr, port, ok := parseSockName(rec.name); ok {
			out = append(out, listenerFromRecord(rec, addr, port, model.ProtocolUDP))
		}
	}
	return out, nil
}
