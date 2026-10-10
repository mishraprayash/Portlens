package platform

import (
	"bytes"
	"net"
	"os"
)

// The proc-net parsing helpers are shared by the linux port/network providers
// but are pure (they read a file and parse text), so they live outside the
// build-tagged files and are unit-testable on any platform. Parsing is
// byte-oriented to keep allocations off the hot path: hex addresses and ports
// are decoded by hand instead of via fmt or per-token string conversions.

type procNetRow struct {
	localAddr  string
	localPort  uint16
	remoteAddr string
	remotePort uint16
	state      string
	inode      uint64
	uid        uint32
}

var tcpStateNames = map[string]string{
	"01": "ESTABLISHED",
	"02": "SYN_SENT",
	"03": "SYN_RECV",
	"04": "FIN_WAIT1",
	"05": "FIN_WAIT2",
	"06": "TIME_WAIT",
	"07": "CLOSE",
	"08": "CLOSE_WAIT",
	"09": "LAST_ACK",
	"0A": "LISTEN",
	"0B": "CLOSING",
}

func parseProcNet(path string) ([]procNetRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseProcNetBytes(data), nil
}

func parseProcNetContent(content string) []procNetRow {
	return parseProcNetBytes([]byte(content))
}

// parseProcNetBytes parses /proc/net/{tcp,udp,tcp6,udp6} content without
// materializing per-token strings. The header line is skipped; each data line
// has ≥10 fields: sl, local, remote, st, tx_queue, rx_queue, tr, tm->when,
// retrnsmt, uid, timeout, inode, ...
func parseProcNetBytes(data []byte) []procNetRow {
	var rows []procNetRow
	rest := data
	first := true
	for {
		line, remaining, ok := nextLine(rest)
		if !ok {
			break
		}
		rest = remaining
		if first {
			first = false
			continue // header
		}
		if len(line) == 0 || line[0] == '\n' {
			continue
		}
		fields := bytes.Fields(line)
		if len(fields) < 10 {
			continue
		}
		local := fields[1]
		remote := fields[2]
		lcolon := bytes.IndexByte(local, ':')
		if lcolon < 0 {
			continue
		}
		lp, lok := parseHexUint(local[lcolon+1:], 16)
		if !lok {
			continue
		}
		rcolon := bytes.IndexByte(remote, ':')
		var rp uint64
		if rcolon >= 0 {
			rp, _ = parseHexUint(remote[rcolon+1:], 16)
		}
		inode, _ := parseDecUint(fields[9], 64)
		uid, _ := parseDecUint(fields[7], 32)

		remoteAddr := ""
		if rcolon >= 0 {
			remoteAddr = decodeAddrBytes(remote[:rcolon])
		}
		rows = append(rows, procNetRow{
			localAddr:  decodeAddrBytes(local[:lcolon]),
			localPort:  uint16(lp),
			remoteAddr: remoteAddr,
			remotePort: uint16(rp),
			state:      string(fields[3]),
			inode:      inode,
			uid:        uint32(uid),
		})
	}
	return rows
}

// nextLine returns the next newline-terminated line (without the newline).
func nextLine(data []byte) (line, rest []byte, ok bool) {
	if len(data) == 0 {
		return nil, nil, false
	}
	i := bytes.IndexByte(data, '\n')
	if i < 0 {
		return data, nil, true
	}
	return data[:i], data[i+1:], true
}

// parseHexUint parses up to maxBits of hexadecimal without allocation.
func parseHexUint(b []byte, maxBits int) (uint64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var v uint64
	for _, c := range b {
		d, ok := hexDigit(c)
		if !ok {
			return 0, false
		}
		v = v<<4 | uint64(d)
	}
	if v>>uint(maxBits) != 0 {
		return 0, false
	}
	return v, true
}

// parseDecUint parses up to maxBits of decimal without allocation.
func parseDecUint(b []byte, maxBits int) (uint64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var v uint64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + uint64(c-'0')
	}
	if v>>uint(maxBits) != 0 {
		return 0, false
	}
	return v, true
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// decodeAddrBytes converts a /proc/net little-endian hex address to a canonical
// IP string. IPv4 addresses are 8 hex chars; IPv6 addresses are 32 hex chars
// stored as four little-endian 32-bit words. IPv4 is formatted by hand; IPv6
// reuses net.IP for correct `::` compression.
func decodeAddrBytes(b []byte) string {
	if len(b) == 8 {
		var raw [4]byte
		raw[0] = nibble(b[6])<<4 | nibble(b[7])
		raw[1] = nibble(b[4])<<4 | nibble(b[5])
		raw[2] = nibble(b[2])<<4 | nibble(b[3])
		raw[3] = nibble(b[0])<<4 | nibble(b[1])
		var buf [16]byte
		n := 0
		for i := 0; i < 4; i++ {
			if i > 0 {
				buf[n] = '.'
				n++
			}
			n += appendDecimal(buf[n:], uint64(raw[i]))
		}
		return string(buf[:n])
	}
	if len(b) == 32 {
		var raw [16]byte
		for w := 0; w < 4; w++ {
			base := w * 8
			raw[w*4+0] = nibble(b[base+6])<<4 | nibble(b[base+7])
			raw[w*4+1] = nibble(b[base+4])<<4 | nibble(b[base+5])
			raw[w*4+2] = nibble(b[base+2])<<4 | nibble(b[base+3])
			raw[w*4+3] = nibble(b[base+0])<<4 | nibble(b[base+1])
		}
		return net.IP(raw[:]).String()
	}
	return ""
}

func nibble(c byte) byte {
	d, _ := hexDigit(c)
	return d
}

// appendDecimal writes v as decimal digits into buf, returning the count.
func appendDecimal(buf []byte, v uint64) int {
	var tmp [20]byte
	i := len(tmp)
	if v == 0 {
		i--
		tmp[i] = '0'
	}
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return copy(buf, tmp[i:])
}

func decodeAddr(hexStr string) string {
	return decodeAddrBytes([]byte(hexStr))
}
