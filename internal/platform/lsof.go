package platform

import (
	"strconv"
	"strings"
)

// lsofField is a single parsed record from lsof's -F field output. These
// parsing helpers are shared by the darwin port/network providers and are
// exercised by unit tests on every platform.
type lsofField struct {
	pid     int32
	command string
	family  string // IPv4 or IPv6
	name    string // e.g. "127.0.0.1:3000" or "*:3000" or "a:b->c:d"
	state   string // from TST=, e.g. LISTEN, ESTABLISHED
}

// parseLsofFields parses lsof -F field output into records. Process-level
// fields (pid, command) precede each block of socket records and are carried
// into each socket entry.
func parseLsofFields(data string) []lsofField {
	var records []lsofField
	var pid int32
	var command string
	var cur lsofField
	haveSock := false

	flush := func() {
		if haveSock && cur.name != "" {
			cur.pid = pid
			cur.command = command
			records = append(records, cur)
		}
		cur = lsofField{}
		haveSock = false
	}

	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "p"):
			if v, err := strconv.ParseInt(line[1:], 10, 32); err == nil {
				pid = int32(v)
			}
		case strings.HasPrefix(line, "c"):
			command = line[1:]
		case strings.HasPrefix(line, "f"):
			flush()
			haveSock = true
		case strings.HasPrefix(line, "t"):
			cur.family = line[1:]
		case strings.HasPrefix(line, "TST="):
			cur.state = line[4:]
		case strings.HasPrefix(line, "n"):
			cur.name = line[1:]
		}
	}
	flush()
	return records
}

// parseSockName splits an lsof socket name into address and port. IPv6
// addresses are wrapped in brackets by lsof; wildcard is "*".
func parseSockName(s string) (addr string, port uint16, ok bool) {
	if s == "" {
		return "", 0, false
	}
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 || !strings.HasPrefix(s[end+1:], ":") {
			return "", 0, false
		}
		addr = s[1:end]
		return parsePort(s[end+2:], addr)
	}
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", 0, false
	}
	return parsePort(s[i+1:], s[:i])
}

func parsePort(p string, addr string) (string, uint16, bool) {
	n, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		return "", 0, false
	}
	return addr, uint16(n), true
}

// normalizeAddr maps lsof's wildcard "*" to a concrete wildcard address based
// on the address family so downstream exposure detection is straightforward.
func normalizeAddr(addr, family string) string {
	if addr == "*" {
		if family == "IPv6" {
			return "::"
		}
		return "0.0.0.0"
	}
	return addr
}
