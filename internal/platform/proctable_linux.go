//go:build linux

package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
)

// loadProcessTable snapshots the process table with a single pass over /proc.
// Each entry is parsed from /proc/<pid>/stat with a byte-oriented parser that
// avoids fmt, regex, and per-field allocations.
func loadProcessTable() ([]processRow, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	rows := make([]processRow, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.ParseInt(e.Name(), 10, 32)
		if err != nil || pid <= 0 {
			continue
		}
		if row, ok := readStatRow(int32(pid)); ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// readStatRow parses /proc/<pid>/stat (pid (comm) state ppid ...). comm may
// contain spaces and parentheses, so the name runs between the first '(' and
// the last ')' and the parent PID is the second token after that.
func readStatRow(pid int32) (processRow, bool) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(int(pid)), "stat"))
	if err != nil {
		return processRow{}, false
	}
	open := bytes.IndexByte(data, '(')
	close := bytes.LastIndexByte(data, ')')
	if open < 0 || close <= open {
		return processRow{}, false
	}
	row := processRow{
		pid:  pid,
		name: string(data[open+1 : close]),
	}
	rest := data[close+1:]
	rest = trimLeftSpace(rest)
	// Skip the state token.
	for len(rest) > 0 && rest[0] != ' ' {
		rest = rest[1:]
	}
	rest = trimLeftSpace(rest)
	// Parse the parent PID.
	var v int64
	for len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		v = v*10 + int64(rest[0]-'0')
		rest = rest[1:]
	}
	row.ppid = int32(v)
	return row, true
}

func trimLeftSpace(b []byte) []byte {
	for len(b) > 0 && b[0] == ' ' {
		b = b[1:]
	}
	return b
}
