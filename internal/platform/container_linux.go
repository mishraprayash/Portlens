package platform

import (
	"fmt"
	"os"
)

// containerIDForPID returns the container ID a process runs inside, or "" when
// the process is not inside a container. On Linux this is derived from the
// process's cgroup, which is a kernel fact: no daemon round-trip is needed.
func containerIDForPID(pid int32) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return ""
	}
	return containerIDFromCgroup(string(data))
}
