//go:build darwin

package platform

import (
	"bytes"
	"context"
	"encoding/binary"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"

	"github.com/portlens/portlens/internal/model"
)

// Native macOS process metadata. Everything is read through kernel interfaces:
// sysctl for identity/state (no FFI), the raw __sysctl syscall for the
// argument list, and libproc (proc_pidpath/proc_pidinfo) only where no sysctl
// exists (executable path resolution and the working directory). No external
// commands are ever spawned.

const (
	ctlKern         = 1  // CTL_KERN
	kernProcargs2   = 49 // KERN_PROCARGS2 — argv + exec path for a pid
	procPidInfoPath = 9  // PROC_PIDVNODEPATHINFO — cwd of a process
	procPidTaskInfo = 4  // PROC_PIDTASKINFO — resident/virtual size
	procPathInfoMax = 4 * 1024
	szomb           = 5 // process state: zombie
)

// vnodePathInfo mirrors struct vnode_fdinfowithpath from sys/proc_info.h: a
// vnode_fdinfo header followed by the cwd path. The field offsets are taken
// from the macOS SDK and validated against gopsutil's implementation.
type vnodePathInfo struct {
	_       [152]byte
	vipPath [1024]byte
	_       [1176]byte
}

// procTaskInfo mirrors struct proc_taskinfo from sys/proc_info.h.
type procTaskInfo struct {
	VirtualSize      uint64
	ResidentSize     uint64
	TotalUser        uint64
	TotalSystem      uint64
	ThreadsUser      uint64
	ThreadsSystem    uint64
	Policy           int32
	Faults           int32
	Pageins          int32
	CowFaults        int32
	MessagesSent     int32
	MessagesReceived int32
	SyscallsMach     int32
	SyscallsUnix     int32
	Csw              int32
	Threadnum        int32
	Numrunning       int32
	Priority         int32
}

type darwinProcessInspector struct{}

func newProcessInspector() ProcessInspector { return darwinProcessInspector{} }

func (darwinProcessInspector) Info(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	return darwinProcInfo(pid, true)
}

func (darwinProcessInspector) InfoBasic(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	return darwinProcInfo(pid, false)
}

func (darwinProcessInspector) Exists(_ context.Context, pid int32) bool {
	return isProcessAlive(pid)
}

// darwinProcInfo assembles a ProcessInfo for a PID. full controls whether the
// slower fields (start time, user, memory) are fetched; the fast path does not
// need them.
func darwinProcInfo(pid int32, full bool) (*model.ProcessInfo, error) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil {
		return nil, ErrProcessNotFound
	}

	info := &model.ProcessInfo{PID: pid, PPID: kp.Eproc.Ppid}
	info.Name = darwinProcName(pid, cstring(kp.Proc.P_comm[:]))

	exe, cmdline, err := darwinArgs(pid)
	if err == nil {
		info.Exe = darwinExePath(pid, exe)
		info.Cmdline = cmdline
		info.Command = commandFromCmdline(cmdline, info.Name)
	}
	if cwd := darwinCwd(pid); cwd != "" {
		info.CWD = cwd
	}

	if full {
		info.StartTime = time.Unix(kp.Proc.P_starttime.Sec, int64(kp.Proc.P_starttime.Usec)*1000)
		info.IsZombie = kp.Proc.P_stat == szomb
		if uid := kp.Eproc.Ucred.Uid; uid > 0 {
			info.User = username(uid)
		}
		if rss := darwinRSS(pid); rss > 0 {
			info.MemoryBytes = rss
		}
	}
	return info, nil
}

// darwinProcName returns the process name, extending the kernel comm (which is
// truncated to 16 bytes) from the argv[0] basename when a longer name is known,
// mirroring the previous gopsutil behavior.
func darwinProcName(pid int32, comm string) string {
	if len(comm) < 15 {
		return comm
	}
	if _, cmdline, err := darwinArgs(pid); err == nil && len(cmdline) > 0 {
		base := filepath.Base(cmdline[0])
		if strings.HasPrefix(base, comm) {
			return base
		}
	}
	return comm
}

// darwinArgs returns the executable path and argv for a PID via the raw
// KERN_PROCARGS2 sysctl — a direct syscall, no external process and no FFI.
func darwinArgs(pid int32) (string, []string, error) {
	buf, err := sysctlKernProcArgs(pid)
	if err != nil {
		return "", nil, err
	}
	// Layout: nargs (int) then the exec path, then argv, then environment —
	// all NUL-terminated. The kernel stores nargs as an int, so skip 8 bytes.
	if len(buf) < 8 {
		return "", nil, ErrProcessNotFound
	}
	nargs := int(binary.LittleEndian.Uint32(buf[:4]))
	rest := bytes.Split(buf[8:], []byte{0})
	if len(rest) == 0 {
		return "", nil, nil
	}
	exe := string(rest[0])
	argv := make([]string, 0, nargs)
	for _, arg := range rest[1:] {
		if len(arg) == 0 {
			continue
		}
		if nargs > 0 {
			argv = append(argv, string(arg))
			nargs--
			continue
		}
		break
	}
	return exe, argv, nil
}

// sysctlKernProcArgs performs the raw __sysctl syscall for KERN_PROCARGS2.
func sysctlKernProcArgs(pid int32) ([]byte, error) {
	mib := []int32{ctlKern, kernProcargs2, pid}
	var length uint64
	_, _, e := unix.Syscall6(unix.SYS_SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		0, uintptr(unsafe.Pointer(&length)), 0, 0)
	if e != 0 {
		return nil, e
	}
	if length == 0 {
		return nil, ErrProcessNotFound
	}
	buf := make([]byte, length)
	_, _, e = unix.Syscall6(unix.SYS_SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&length)), 0, 0)
	if e != 0 {
		return nil, e
	}
	return buf[:length], nil
}

// libproc (proc_pidpath / proc_pidinfo) is used only where no sysctl exists:
// executable path resolution and the working directory. The call pattern
// mirrors the one proven by gopsutil and validated here: register the symbol
// on the current OS thread immediately before the call, and read the result
// before closing the library. The output buffer must live in the same stack
// frame as the FFI call — computing its address earlier (or letting the frame
// grow across the bridge) returns a success code with an empty result.
func libprocPath() (uintptr, func(), error) {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, nil, err
	}
	runtime.LockOSThread()
	return lib, func() {
		purego.Dlclose(lib)
		runtime.UnlockOSThread()
	}, nil
}

func libprocPIDInfoFn(lib uintptr) func(pid, flavor int32, arg uint64, buf uintptr, bufsize int32) int32 {
	var fn func(pid, flavor int32, arg uint64, buf uintptr, bufsize int32) int32
	purego.RegisterLibFunc(&fn, lib, "proc_pidinfo")
	return fn
}

func libprocPIDPathFn(lib uintptr) func(pid int32, buf uintptr, bufsize uint32) int32 {
	var fn func(pid int32, buf uintptr, bufsize uint32) int32
	purego.RegisterLibFunc(&fn, lib, "proc_pidpath")
	return fn
}

// darwinExePath resolves the executable path via libproc's proc_pidpath,
// falling back to the argv[0] exec path when libproc is unavailable.
func darwinExePath(pid int32, argvExe string) string {
	lib, close, err := libprocPath()
	if err != nil {
		return argvExe
	}
	defer close()
	fn := libprocPIDPathFn(lib)
	buf := make([]byte, procPathInfoMax)
	n := fn(pid, uintptr(unsafe.Pointer(&buf[0])), uint32(len(buf)))
	if n <= 0 || n >= int32(len(buf)) {
		return argvExe
	}
	return cstring(buf[:n])
}

// darwinCwd returns the working directory via libproc's proc_pidinfo
// (PROC_PIDVNODEPATHINFO), or "" when unavailable or not permitted.
func darwinCwd(pid int32) string {
	lib, close, err := libprocPath()
	if err != nil {
		return ""
	}
	defer close()
	fn := libprocPIDInfoFn(lib)
	var vpi vnodePathInfo
	const size = int32(unsafe.Sizeof(vpi))
	n := fn(pid, procPidInfoPath, 0, uintptr(unsafe.Pointer(&vpi)), size)
	if n != size {
		return ""
	}
	return cstring(vpi.vipPath[:])
}

// darwinRSS returns the resident set size via libproc's PROC_PIDTASKINFO.
func darwinRSS(pid int32) uint64 {
	lib, close, err := libprocPath()
	if err != nil {
		return 0
	}
	defer close()
	fn := libprocPIDInfoFn(lib)
	var ti procTaskInfo
	const size = int32(unsafe.Sizeof(ti))
	n := fn(pid, procPidTaskInfo, 0, uintptr(unsafe.Pointer(&ti)), size)
	if n != size {
		return 0
	}
	return ti.ResidentSize
}

var (
	userMu  sync.Mutex
	userMap = map[uint32]string{}
)

// username resolves a numeric uid to a login name with a per-invocation cache.
func username(uid uint32) string {
	userMu.Lock()
	defer userMu.Unlock()
	if name, ok := userMap[uid]; ok {
		return name
	}
	name := ""
	if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
		name = u.Username
	}
	userMap[uid] = name
	return name
}

// isProcessAlive reports whether a PID is live and not a zombie, using the
// kernel's process table directly.
func isProcessAlive(pid int32) bool {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil {
		return false
	}
	return kp.Proc.P_stat != szomb
}
