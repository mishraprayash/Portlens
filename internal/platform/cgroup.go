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
	offset := 0
	for {
		idx := containerIDToken.FindStringIndex(line[offset:])
		if idx == nil {
			return ""
		}
		start := offset + idx[0]
		end := offset + idx[1]
		if !hexNeighbor(line, start, end) {
			return line[start:end]
		}
		offset = end
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
