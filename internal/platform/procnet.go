package platform

import (
	"encoding/hex"
	"net"
	"os"
	"strconv"
	"strings"
)

// The proc-net parsing helpers are shared by the linux port/network providers
// but are pure (they read a file and parse text), so they live outside the
// build-tagged files and are unit-testable on any platform.

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
	return parseProcNetContent(string(data)), nil
}

func parseProcNetContent(content string) []procNetRow {
	lines := strings.Split(content, "\n")
	var rows []procNetRow
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		local := strings.Split(fields[1], ":")
		remote := strings.Split(fields[2], ":")
		if len(local) != 2 || len(remote) != 2 {
			continue
		}
		lp, err := strconv.ParseUint(local[1], 16, 16)
		if err != nil {
			continue
		}
		rp, _ := strconv.ParseUint(remote[1], 16, 16)
		inode, _ := strconv.ParseUint(fields[9], 10, 64)
		uid, _ := strconv.ParseUint(fields[7], 10, 32)
		rows = append(rows, procNetRow{
			localAddr:  decodeAddr(local[0]),
			localPort:  uint16(lp),
			remoteAddr: decodeAddr(remote[0]),
			remotePort: uint16(rp),
			state:      fields[3],
			inode:      inode,
			uid:        uint32(uid),
		})
	}
	return rows
}

// decodeAddr converts the /proc/net little-endian hex address to a canonical
// IP string. IPv4 addresses are 8 hex chars; IPv6 addresses are 32 hex chars
// stored as four little-endian 32-bit words.
func decodeAddr(hexStr string) string {
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return ""
	}
	if len(raw) == 4 {
		for i, j := 0, len(raw)-1; i < j; i, j = i+1, j-1 {
			raw[i], raw[j] = raw[j], raw[i]
		}
	} else if len(raw) == 16 {
		for i := 0; i < len(raw); i += 4 {
			raw[i], raw[i+3] = raw[i+3], raw[i]
			raw[i+1], raw[i+2] = raw[i+2], raw[i+1]
		}
	}
	return net.IP(raw).String()
}
