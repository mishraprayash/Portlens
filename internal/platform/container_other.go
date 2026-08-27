//go:build !linux

package platform

// containerIDForPID returns "" on non-Linux platforms: host processes are not
// members of the container runtime's cgroups there (for example, on macOS the
// Docker VM is a single host process), so a cgroup lookup is meaningless.
func containerIDForPID(pid int32) string {
	return ""
}
