package platform

import (
	"regexp"
	"strconv"
	"strings"
)

// containerIDLength is the length of a docker/containerd container ID.
const containerIDLength = 64

var containerIDToken = regexp.MustCompile("[a-f0-9]{" + strconv.Itoa(containerIDLength) + "}")

// containerIDFromCgroup extracts a container ID from a /proc/<pid>/cgroup
// file's contents. Docker and containerd embed the 64-hex container ID in the
// cgroup path (e.g. "/docker/<id>" or "/system.slice/docker-<id>.scope").
// A token is only accepted when it is not embedded in a longer hex string, so
// unrelated 64-hex identifiers are not mistaken for container IDs.
func containerIDFromCgroup(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if id := scanContainerID(line); id != "" {
			return id
		}
	}
	return ""
}

func scanContainerID(line string) string {
	for {
		idx := containerIDToken.FindStringIndex(line)
		if idx == nil {
			return ""
		}
		if !hexNeighbor(line, idx[0], idx[1]) {
			return line[idx[0]:idx[1]]
		}
		line = line[idx[1]:]
	}
}

func hexNeighbor(line string, start, end int) bool {
	leftHex := start > 0 && isHexByte(line[start-1])
	rightHex := end < len(line) && isHexByte(line[end])
	return leftHex || rightHex
}

func isHexByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
