package platform

import (
	"errors"

	"github.com/mishraprayash/Portlens/internal/model"
)

// ErrProcessNotFound is returned when a PID does not exist (or has exited).
var ErrProcessNotFound = errors.New("process not found")

// ErrPermissionDenied is returned when the platform refuses access to data
// that requires elevated privileges. PortLens never attempts to escalate.
// It is the model sentinel so model.MapExitCode maps it to
// exitcode.PermissionDenied (4) as documented in docs/exit-codes.md.
var ErrPermissionDenied = model.ErrPermissionDenied
