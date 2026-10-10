package detect

import (
	"strings"

	"github.com/portlens/portlens/internal/model"
)

// userPathPrefixes are locations where user-installed software lives (Homebrew,
// /opt, user home, and app bundles on macOS). A process running from here is
// third-party, never a native OS component.
var userPathPrefixes = []string{
	"/opt/homebrew/",
	"/usr/local/",
	"/opt/",
	"/Applications/",
	"/Users/",
}

// systemPathPrefixes are locations owned by the operating system. On macOS
// these hold the OS and its daemons; on Linux they hold the base distribution.
var systemPathPrefixes = []string{
	"/System/",
	"/Library/Apple/",
	"/usr/libexec/",
	"/usr/sbin/",
	"/sbin/",
	"/usr/bin/",
	"/bin/",
	"/usr/lib/",
}

// systemProcessNames are well-known OS daemons whose executable path is not
// always resolvable (e.g. launchd jobs or kernel threads); the name alone is a
// reliable system signal for these.
var systemProcessNames = map[string]bool{
	"launchd": true, "kernel_task": true, "mDNSResponder": true, "kdc": true,
	"syslogd": true, "configd": true, "notifyd": true, "cfprefsd": true,
	"taskgated": true, "securityd": true, "logd": true, "symptomsd": true,
	"rapportd": true, "identityservicesd": true, "sharingd": true,
	"replicatord": true, "airportd": true, "coreaudiod": true, "hidd": true,
	"sandboxd": true, "nfsd": true, "automountd": true, "screensharingd": true,
	"runningboardd": true, "systemd": true, "systemd-journald": true,
	"init": true, "kthreadd": true, "cron": true, "rsyslogd": true,
}

// userProcessNames are well-known third-party daemons that are installed by
// the user (Homebrew, a language toolchain, a vendor package) and never ship
// as native OS components. The name alone is a reliable "user" signal for
// these, and it covers the case where the executable path cannot be resolved
// (e.g. a process owned by another user on macOS).
var userProcessNames = map[string]bool{
	"postgres": true, "postmaster": true, "redis-server": true, "mongod": true,
	"mysqld": true, "mariadbd": true, "nginx": true, "httpd": true,
	"memcached": true, "elasticsearch": true, "kafka": true, "node": true,
	"deno": true, "bun": true, "docker": true, "dockerd": true,
	"containerd": true, "colima": true, "podman": true, "java": true,
	"python": true, "python3": true, "ruby": true, "php": true,
}

// ProcessOrigin classifies a process as a native operating-system component
// ("system") or a user-installed/third-party program ("user"). It returns the
// empty Origin when the classification is unknown. The decision is a heuristic
// based on the executable path and process name, so it is presented as an
// inference, never as a fact.
func ProcessOrigin(p *model.ProcessInfo) model.Origin {
	if p == nil {
		return ""
	}
	if exe := p.Exe; exe != "" {
		for _, pre := range userPathPrefixes {
			if strings.HasPrefix(exe, pre) {
				return model.OriginUser
			}
		}
		for _, pre := range systemPathPrefixes {
			if strings.HasPrefix(exe, pre) {
				return model.OriginSystem
			}
		}
	}
	name := strings.ToLower(p.Name)
	if systemProcessNames[name] {
		return model.OriginSystem
	}
	if userProcessNames[name] {
		return model.OriginUser
	}
	return ""
}
