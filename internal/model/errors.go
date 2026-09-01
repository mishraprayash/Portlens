// Package model defines core domain models and sentinel errors.
package model

import (
	"errors"
	"os"

	"github.com/portlens/portlens/internal/exitcode"
)

var (
	// ErrPortNotFound indicates no process is actively listening on the requested port.
	ErrPortNotFound = errors.New("no process is listening on this port")

	// ErrPermissionDenied indicates the operation requires elevated privileges.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrProcessActionFailed indicates a process lifecycle action (kill, restart, open) failed.
	ErrProcessActionFailed = errors.New("process action failed")

	// ErrInvalidPort indicates a port outside 1-65535 or a malformed port range.
	ErrInvalidPort = errors.New("invalid port")

	// ErrInvalidArguments indicates invalid or missing command arguments.
	ErrInvalidArguments = errors.New("invalid arguments")
)

// MapExitCode maps an error to an appropriate process exit code using errors.Is.
func MapExitCode(err error) int {
	if err == nil {
		return exitcode.Success
	}
	switch {
	case errors.Is(err, ErrPortNotFound):
		return exitcode.PortNotFound
	case errors.Is(err, ErrPermissionDenied), os.IsPermission(err):
		return exitcode.PermissionDenied
	case errors.Is(err, ErrProcessActionFailed):
		return exitcode.ProcessActionFailed
	case errors.Is(err, ErrInvalidPort), errors.Is(err, ErrInvalidArguments):
		return exitcode.InvalidArguments
	default:
		return exitcode.GeneralError
	}
}
