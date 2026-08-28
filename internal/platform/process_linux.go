//go:build linux

package platform

import (
	"bytes"
	"context"
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/portlens/portlens/internal/model"
)

// Native Linux process metadata via /proc. Byte-oriented parsing keeps the hot
// path allocation-light and free of fmt and external commands.

type linuxProcessInspector struct{}

func newProcessInspector() ProcessInspector { return linuxProcessInspector{} }

func (linuxProcessInspector) Info(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	return linuxProcInfo(pid, true)
}

func (linuxProcessInspector) InfoBasic(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	return linuxProcInfo(pid, false)
}

func (linuxProcessInspector) Exists(_ context.Context, pid int32) bool {
	return isProcessAlive(pid)
}

// statRow is the parsed subset of /proc/<pid>/stat that process metadata needs.
type statRow struct {
	name   string
	ppid   int32
	zombie bool
}

var (
	bootOnce sync.Once
	bootTime time.Time
)

// linuxBootTime reads the boot time (btime) from /proc/stat once per
// invocation; it is combined with each process's start ticks.
func linuxBootTime() time.Time {
	bootOnce.Do(func() {
		if data, err := os.ReadFile("/proc/stat"); err == nil {
			for _, line := range bytes.Split(data, []byte{'\n'}) {
				if bytes.HasPrefix(line, []byte("btime ")) {
					if n, err := strconv.ParseInt(string(line[6:]), 10, 64); err == nil {
						bootTime = time.Unix(n, 0)
					}
					break
				}
			}
		}
	})
	return bootTime
}

func procDir(pid int32) string {
	return "/proc/" + strconv.Itoa(int(pid))
}

// linuxProcInfo assembles a ProcessInfo for a PID. full controls whether the
// slower fields (start time, user, memory) are fetched.
func linuxProcInfo(pid int32, full bool) (*model.ProcessInfo, error) {
	info := &model.ProcessInfo{PID: pid}
	dir := procDir(pid)

	stat, err := os.ReadFile(dir + "/stat")
	if err != nil {
		return nil, ErrProcessNotFound
	}
	row, ok := parseStatRow(stat)
	if !ok {
		return nil, ErrProcessNotFound
	}
	info.Name = row.name
	info.PPID = row.ppid
	info.IsZombie = row.zombie

	if cmdline, err := os.ReadFile(dir + "/cmdline"); err == nil {
		info.Cmdline = splitNUL(cmdline)
		if len(info.Cmdline) > 0 {
			info.Exe = info.Cmdline[0]
		}
		info.Command = commandFromCmdline(info.Cmdline, info.Name)
	}
	if exe, err := os.Readlink(dir + "/exe"); err == nil {
		info.Exe = exe
	}
	if cwd, err := os.Readlink(dir + "/cwd"); err == nil {
		info.CWD = cwd
	}

	if full {
		if st, ok := linuxStartTime(pid); ok {
			info.StartTime = st
		}
		info.User = linuxUser(pid)
		if rss := linuxRSS(pid); rss > 0 {
			info.MemoryBytes = rss
		}
	}
	return info, nil
}

// parseStatRow parses /proc/<pid>/stat (pid (comm) state ppid ...). comm may
// contain spaces and parentheses, so the name runs between the first '(' and
// the last ')'. Returns the name, ppid, and whether the state is 'Z'.
func parseStatRow(data []byte) (statRow, bool) {
	var row statRow
	open := bytes.IndexByte(data, '(')
	close := bytes.LastIndexByte(data, ')')
	if open < 0 || close <= open {
		return row, false
	}
	row.name = string(data[open+1 : close])
	rest := trimLeftSpace(data[close+1:])
	// State is the first token after ')'; 'Z' means zombie.
	if len(rest) > 0 {
		row.zombie = rest[0] == 'Z'
	}
	rest = trimLeftSpace(rest[1:])
	// Parent PID is the second token after ')'.
	var v int64
	for len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		v = v*10 + int64(rest[0]-'0')
		rest = rest[1:]
	}
	row.ppid = int32(v)
	return row, true
}

// linuxStartTime converts a process's start ticks (field 22 of stat) plus boot
// time into a wall-clock start time. Linux schedules in USER_HZ (100) clock
// ticks per second on essentially all platforms.
func linuxStartTime(pid int32) (time.Time, bool) {
	const userHz = 100
	data, err := os.ReadFile(procDir(pid) + "/stat")
	if err != nil {
		return time.Time{}, false
	}
	open := bytes.IndexByte(data, '(')
	close := bytes.LastIndexByte(data, ')')
	if open < 0 || close <= open {
		return time.Time{}, false
	}
	rest := data[close+1:]
	// starttime is field 22 → skip 19 tokens (fields 3..21).
	for i := 0; i < 19; i++ {
		rest = skipToken(rest)
		if len(rest) == 0 {
			return time.Time{}, false
		}
	}
	rest = trimLeftSpace(rest)
	var ticks int64
	for len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		ticks = ticks*10 + int64(rest[0]-'0')
		rest = rest[1:]
	}
	if ticks <= 0 {
		return time.Time{}, false
	}
	hz := int64(userHz)
	sec := linuxBootTime().Unix() + ticks/hz
	nsec := (ticks % hz) * int64(time.Second/time.Nanosecond) / hz
	return time.Unix(sec, nsec), true
}

func skipToken(b []byte) []byte {
	b = trimLeftSpace(b)
	for len(b) > 0 && b[0] != ' ' {
		b = b[1:]
	}
	return b
}

var (
	userMu  sync.Mutex
	userMap = map[uint32]string{}
)

// linuxUser resolves the owning user name from /proc/<pid>/status, cached per
// invocation.
func linuxUser(pid int32) string {
	data, err := os.ReadFile(procDir(pid) + "/status")
	if err != nil {
		return ""
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("Uid:")) {
			continue
		}
		f := bytes.Fields(line[4:])
		if len(f) == 0 {
			return ""
		}
		return uidToName(f[0])
	}
	return ""
}

func uidToName(uidBytes []byte) string {
	uid, err := strconv.ParseUint(string(uidBytes), 10, 32)
	if err != nil {
		return ""
	}
	userMu.Lock()
	defer userMu.Unlock()
	if name, ok := userMap[uint32(uid)]; ok {
		return name
	}
	name := ""
	if u, err := user.LookupId(strconv.FormatUint(uid, 10)); err == nil {
		name = u.Username
	}
	userMap[uint32(uid)] = name
	return name
}

// linuxRSS reads the resident set size (in bytes) from /proc/<pid>/statm.
func linuxRSS(pid int32) uint64 {
	data, err := os.ReadFile(procDir(pid) + "/statm")
	if err != nil {
		return 0
	}
	// statm: size resident shared text lib data dt — resident pages is field 2.
	f := bytes.Fields(data)
	if len(f) < 2 {
		return 0
	}
	pages, err := strconv.ParseUint(string(f[1]), 10, 64)
	if err != nil {
		return 0
	}
	return pages * uint64(unix.Getpagesize())
}

// isProcessAlive reports whether a PID is live and not a zombie.
func isProcessAlive(pid int32) bool {
	if err := syscall.Kill(int(pid), 0); err != nil {
		return false
	}
	data, err := os.ReadFile(procDir(pid) + "/stat")
	if err != nil {
		return false
	}
	open := bytes.IndexByte(data, '(')
	close := bytes.LastIndexByte(data, ')')
	if open < 0 || close <= open || close+1 >= len(data) {
		return true
	}
	return data[close+1] != 'Z'
}

// splitNUL splits a NUL-separated byte slice into its non-empty parts.
func splitNUL(b []byte) []string {
	var out []string
	for _, part := range bytes.Split(b, []byte{0}) {
		if len(part) > 0 {
			out = append(out, string(part))
		}
	}
	return out
}
