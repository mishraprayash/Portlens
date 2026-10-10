package platform

import "errors"

// ErrProcessNotFound is returned when a PID does not exist (or has exited).
var ErrProcessNotFound = errors.New("process not found")

// ErrPermissionDenied is returned when the platform refuses access to data that
// requires elevated privileges. PortLens never attempts to escalate.
var ErrPermissionDenied = errors.New("permission denied")
