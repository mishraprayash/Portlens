//go:build darwin

package platform

import (
	"bytes"

	"golang.org/x/sys/unix"
)

// loadProcessTable snapshots the process table with a single sysctl call,
// avoiding the per-process syscalls (and the process spawns) that process
// enumeration libraries incur on macOS.
func loadProcessTable() ([]processRow, error) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	rows := make([]processRow, 0, len(procs))
	for i := range procs {
		kp := &procs[i]
		pid := kp.Proc.P_pid
		if pid <= 0 {
			continue
		}
		rows = append(rows, processRow{
			pid:  pid,
			ppid: kp.Eproc.Ppid,
			name: cstring(kp.Proc.P_comm[:]),
		})
	}
	return rows, nil
}

// cstring converts a fixed-size NUL-terminated byte array to a string.
func cstring(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}
