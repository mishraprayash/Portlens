package platform

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/portlens/portlens/internal/model"
)

// maxDepth guards against pathological process hierarchies (PPID cycles).
const maxDepth = 64

// gopsutilProcessInspector inspects processes via gopsutil, which itself uses
// native system APIs (sysctl on macOS, /proc on Linux). This implementation is
// shared across platforms because gopsutil already provides the OS abstraction
// for process metadata.
type gopsutilProcessInspector struct{}

// gopsutilTreeProvider builds process trees using the same metadata source.
type gopsutilTreeProvider struct {
	inspector ProcessInspector
}

// syscallProcessController sends signals via syscall.Kill.
type syscallProcessController struct{}

func newProcessInspector() ProcessInspector { return gopsutilProcessInspector{} }
func newProcessTreeProvider() ProcessTreeProvider {
	return &gopsutilTreeProvider{inspector: gopsutilProcessInspector{}}
}
func newProcessController() ProcessController { return syscallProcessController{} }

func (gopsutilProcessInspector) Info(_ context.Context, pid int32) (*model.ProcessInfo, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, ErrProcessNotFound
	}
	return infoFromProcess(p)
}

func infoFromProcess(p *process.Process) (*model.ProcessInfo, error) {
	info := &model.ProcessInfo{PID: p.Pid}

	if name, err := p.Name(); err == nil {
		info.Name = name
	}
	if exe, err := p.Exe(); err == nil {
		info.Exe = exe
	}
	if cmdline, err := p.CmdlineSlice(); err == nil {
		info.Cmdline = cmdline
		info.Command = commandFromCmdline(cmdline, info.Name)
	} else if cmdline, err := p.Cmdline(); err == nil && cmdline != "" {
		info.Command = cmdline
	}
	if cwd, err := p.Cwd(); err == nil {
		info.CWD = cwd
	}
	if ppid, err := p.Ppid(); err == nil {
		info.PPID = ppid
	}
	if ct, err := p.CreateTime(); err == nil {
		info.StartTime = time.UnixMilli(ct)
	}
	if u, err := p.Username(); err == nil {
		info.User = u
	}
	if term, err := p.Terminal(); err == nil {
		info.Terminal = term
	}
	if mem, err := p.MemoryInfo(); err == nil {
		info.MemoryBytes = mem.RSS
	}
	if status, err := p.Status(); err == nil {
		for _, s := range status {
			if strings.EqualFold(s, "z") || strings.EqualFold(s, "zombie") {
				info.IsZombie = true
				break
			}
		}
	}
	// CPU percentage is expensive (requires two samples) and best-effort only.
	return info, nil
}

func (gopsutilProcessInspector) Children(_ context.Context, pid int32) ([]*model.ProcessInfo, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, ErrProcessNotFound
	}
	kids, err := p.Children()
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProcessInfo, 0, len(kids))
	for _, k := range kids {
		if info, err := infoFromProcess(k); err == nil {
			out = append(out, info)
		}
	}
	return out, nil
}

func (gopsutilProcessInspector) Exists(_ context.Context, pid int32) bool {
	return isProcessAlive(pid)
}

func (t *gopsutilTreeProvider) Ancestors(ctx context.Context, pid int32) ([]*model.ProcessInfo, error) {
	var chain []*model.ProcessInfo
	seen := map[int32]bool{}
	cur := pid
	for depth := 0; depth < maxDepth; depth++ {
		if seen[cur] {
			break
		}
		seen[cur] = true
		info, err := t.inspector.Info(ctx, cur)
		if err != nil {
			break
		}
		chain = append(chain, info)
		if info.PPID <= 0 || info.PPID == cur {
			break
		}
		cur = info.PPID
	}
	// chain is child→parent; reverse so oldest comes first.
	reverseInfos(chain)
	return chain, nil
}

func (t *gopsutilTreeProvider) Descendants(ctx context.Context, pid int32) (*model.ProcessTree, error) {
	info, err := t.inspector.Info(ctx, pid)
	if err != nil {
		return nil, err
	}
	root := &model.ProcessTree{Process: *info}
	var walk func(node *model.ProcessTree, current int32, depth int, seen map[int32]bool)
	walk = func(node *model.ProcessTree, current int32, depth int, seen map[int32]bool) {
		if depth >= maxDepth || seen[current] {
			return
		}
		seen[current] = true
		kids, err := t.inspector.Children(ctx, current)
		if err != nil {
			return
		}
		for _, k := range kids {
			child := &model.ProcessTree{Process: *k}
			node.Children = append(node.Children, child)
			walk(child, k.PID, depth+1, seen)
		}
	}
	walk(root, pid, 0, map[int32]bool{})
	return root, nil
}

func reverseInfos(s []*model.ProcessInfo) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func (syscallProcessController) Signal(_ context.Context, pid int32, sig Signal) error {
	var s syscall.Signal
	switch sig {
	case SignalTerm:
		s = syscall.SIGTERM
	case SignalKill:
		s = syscall.SIGKILL
	case SignalInterrupt:
		s = syscall.SIGINT
	default:
		return syscall.EINVAL
	}
	if err := syscall.Kill(int(pid), s); err != nil {
		if err == syscall.ESRCH {
			return ErrProcessNotFound
		}
		if err == syscall.EPERM {
			return ErrPermissionDenied
		}
		return err
	}
	return nil
}

func (syscallProcessController) IsAlive(_ context.Context, pid int32) bool {
	return isProcessAlive(pid)
}

// isProcessAlive reports whether a PID is still a live process. Zombie
// processes (which linger after exiting until reaped) are considered dead.
func isProcessAlive(pid int32) bool {
	p, err := process.NewProcess(pid)
	if err != nil {
		return false
	}
	if status, err := p.Status(); err == nil {
		for _, s := range status {
			if strings.EqualFold(s, "z") || strings.EqualFold(s, "zombie") {
				return false
			}
		}
	}
	return true
}

// commandFromCmdline renders a human-friendly command line: the executable path
// is reduced to its basename and, for interpreter wrappers (node, python, ...),
// a known tool is substituted for the interpreter (e.g. "pnpm dev" instead of
// "node /path/to/pnpm.cjs dev").
func commandFromCmdline(cmdline []string, name string) string {
	if len(cmdline) == 0 {
		return name
	}
	prog := filepath.Base(cmdline[0])
	if prog == "" {
		prog = name
	}
	args := cmdline[1:]
	if isInterpreter(prog) && len(args) > 0 {
		if tool := knownTool(args[0]); tool != "" {
			prog = tool
			args = args[1:]
		}
	}
	args = stripLaunchEnv(args)
	return strings.TrimSpace(strings.Join(append([]string{prog}, args...), " "))
}

// stripLaunchEnv removes launchd/systemd-style environment assignments that
// get appended to a process's argv (e.g. "XPC_FLAGS=1", "LOGNAME=user"). These
// are launch artifacts, not meaningful command arguments, so they are omitted
// from the human-friendly command while remaining in the raw Cmdline slice.
func stripLaunchEnv(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if isEnvAssignment(a) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func isEnvAssignment(s string) bool {
	if s == "" || s[0] == '-' {
		return false
	}
	eq := strings.IndexByte(s, '=')
	if eq <= 0 || eq == len(s)-1 {
		return false
	}
	key := s[:eq]
	for i := 0; i < len(key); i++ {
		c := key[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return key[0] >= 'A' && key[0] <= 'Z'
}

func isInterpreter(prog string) bool {
	switch strings.ToLower(prog) {
	case "node", "python", "python3", "ruby", "java", "sh", "bash", "zsh":
		return true
	}
	return false
}

var knownToolNames = map[string]bool{
	"pnpm": true, "npm": true, "yarn": true, "npx": true, "nest": true,
	"vite": true, "ts-node": true, "tsx": true, "nodemon": true, "next": true,
	"nuxt": true, "webpack": true, "esbuild": true, "mocha": true, "jest": true,
	"vitest": true, "gulp": true, "pm2": true, "uvicorn": true, "gunicorn": true,
	"flask": true, "django-admin": true, "pip": true, "pip3": true, "pipenv": true,
	"poetry": true, "prisma": true, "expo": true, "ng": true, "vue": true,
}

func knownTool(arg string) string {
	base := filepath.Base(arg)
	base = strings.TrimSuffix(strings.TrimSuffix(base, ".cjs"), ".js")
	base = strings.TrimPrefix(base, "@")
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if knownToolNames[strings.ToLower(base)] {
		return base
	}
	return ""
}
