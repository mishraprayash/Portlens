package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/mishraprayash/Portlens/internal/config"
	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/platform"
	"github.com/mishraprayash/Portlens/internal/version"
)

// Check statuses reported by `portlens doctor`.
const (
	checkOK   = "ok"   // working
	checkWarn = "warn" // a feature is unavailable or degraded
	checkFail = "fail" // core functionality is broken
	checkSkip = "skip" // optional integration not present
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

// doctorEnv abstracts the host so the checks are testable.
type doctorEnv struct {
	goos, goarch string
	euid         int
	lookPath     func(string) (string, error)
	listeners    func(context.Context) (total, unowned int, err error)
	sockets      []string
	dialSocket   func(path string) error
	configCheck  func() (path string, err error)
}

func defaultDoctorEnv() doctorEnv {
	return doctorEnv{
		goos:     runtime.GOOS,
		goarch:   runtime.GOARCH,
		euid:     os.Geteuid(),
		lookPath: exec.LookPath,
		listeners: func(ctx context.Context) (int, int, error) {
			ls, err := platform.New().Ports.Listeners(ctx)
			if err != nil {
				return 0, 0, err
			}
			unowned := 0
			for _, l := range ls {
				if l.PID <= 0 {
					unowned++
				}
			}
			return len(ls), unowned, nil
		},
		sockets: platform.ContainerSocketPaths(),
		dialSocket: func(path string) error {
			c, err := net.DialTimeout("unix", path, 300*time.Millisecond)
			if err != nil {
				return err
			}
			return c.Close()
		},
		configCheck: func() (string, error) {
			_, err := config.Load()
			return config.Path(), err
		},
	}
}

// runDoctorChecks evaluates every check against env.
func runDoctorChecks(ctx context.Context, env doctorEnv) []doctorCheck {
	var out []doctorCheck
	add := func(name, status, detail, hint string) {
		out = append(out, doctorCheck{Name: name, Status: status, Detail: detail, Hint: hint})
	}

	if env.goos == "darwin" || env.goos == "linux" {
		add("platform", checkOK, fmt.Sprintf("%s/%s, portlens %s", env.goos, env.goarch, version.Version), "")
	} else {
		add("platform", checkFail, fmt.Sprintf("%s/%s is not supported", env.goos, env.goarch), "PortLens supports macOS and Linux")
	}

	if env.goos == "darwin" {
		if p, err := env.lookPath("lsof"); err != nil {
			add("port source", checkFail, "lsof not found", "lsof ships with macOS; check your PATH")
		} else {
			add("port source", checkOK, "lsof at "+p, "")
		}
	}
	total, unowned, err := env.listeners(ctx)
	switch {
	case err != nil:
		add("listing ports", checkFail, err.Error(), "")
	case unowned > 0:
		add("listing ports", checkWarn, fmt.Sprintf("%d listeners, %d with an unknown owner", total, unowned),
			"processes owned by other users are hidden without elevated privileges; PortLens never escalates on its own")
	default:
		add("listing ports", checkOK, fmt.Sprintf("%d listeners, all owners resolved", total), "")
	}

	optional := func(name string, tools []string, hint string) {
		for _, t := range tools {
			if p, err := env.lookPath(t); err == nil {
				add(name, checkOK, t+" at "+p, "")
				return
			}
		}
		add(name, checkWarn, "none of "+fmt.Sprint(tools)+" found", hint)
	}
	if env.goos == "darwin" {
		optional("clipboard", []string{"pbcopy"}, "copy actions will be unavailable")
		optional("open in browser", []string{"open"}, "the open action will be unavailable")
		optional("notifications", []string{"osascript"}, "watch --notify will be unavailable")
	} else {
		optional("clipboard", []string{"wl-copy", "xclip", "xsel"}, "install wl-clipboard or xclip for copy actions")
		optional("open in browser", []string{"xdg-open", "sensible-browser", "x-www-browser"}, "install xdg-utils for the open action")
		optional("notifications", []string{"notify-send"}, "install libnotify-bin for watch --notify")
	}

	found := false
	for _, s := range env.sockets {
		if _, err := os.Stat(s); err != nil {
			continue
		}
		if err := env.dialSocket(s); err != nil {
			add("container runtime", checkWarn, fmt.Sprintf("socket %s exists but is not reachable: %v", s, err),
				"check that the daemon is running and you have permission to use the socket")
			found = true
			break
		}
		add("container runtime", checkOK, "daemon reachable at "+s, "")
		found = true
		break
	}
	if !found {
		add("container runtime", checkSkip, "no Docker/Podman socket found; container awareness is off", "")
	}

	if path, err := env.configCheck(); err != nil {
		add("config", checkFail, err.Error(), "fix or delete "+path)
	} else {
		add("config", checkOK, path, "")
	}

	if env.euid == 0 {
		add("privileges", checkWarn, "running as root", "kill/restart actions will act with root privileges; prefer a normal user")
	} else {
		add("privileges", checkOK, "running as an unprivileged user", "")
	}
	return out
}

// doctorExitCode is Success unless a core check failed.
func doctorExitCode(checks []doctorCheck) int {
	for _, c := range checks {
		if c.Status == checkFail {
			return exitcode.GeneralError
		}
	}
	return exitcode.Success
}

func runDoctor(ctx context.Context, stdout io.Writer, jsonOut, color bool, env doctorEnv) int {
	checks := runDoctorChecks(ctx, env)
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(checks)
		return doctorExitCode(checks)
	}
	paint := func(status, s string) string {
		if !color {
			return s
		}
		code := map[string]string{checkOK: "32", checkWarn: "33", checkFail: "31", checkSkip: "2"}[status]
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	for _, c := range checks {
		_, _ = fmt.Fprintf(stdout, "%s  %-18s %s\n", paint(c.Status, fmt.Sprintf("%-4s", c.Status)), c.Name, c.Detail)
		if c.Hint != "" && c.Status != checkOK {
			_, _ = fmt.Fprintf(stdout, "      %-18s ↳ %s\n", "", c.Hint)
		}
	}
	return doctorExitCode(checks)
}

// doctorSubcommand handles `portlens doctor [--json]`.
type doctorSubcommand struct{}

func (c *doctorSubcommand) Name() string      { return "doctor" }
func (c *doctorSubcommand) Aliases() []string { return nil }
func (c *doctorSubcommand) Description() string {
	return "Check which PortLens features work on this machine"
}
func (c *doctorSubcommand) Run(ctx context.Context, args []string, preFlags []string, stdout, stderr io.Writer, _ io.Reader) int {
	jsonOut, noColor := false, false
	for _, a := range append(append([]string(nil), preFlags...), args...) {
		switch a {
		case "-h", "--help":
			printDoctorUsage(stdout)
			return exitcode.Success
		case "-j", "--json":
			jsonOut = true
		case "--no-color":
			noColor = true
		default:
			return fail(stderr, exitcode.InvalidArguments, "portlens doctor: unknown argument %q\nRun 'portlens doctor --help' for usage.\n", a)
		}
	}
	color := !noColor && os.Getenv("NO_COLOR") == "" && isTerminal(stdout)
	return runDoctor(ctx, stdout, jsonOut, color, defaultDoctorEnv())
}
