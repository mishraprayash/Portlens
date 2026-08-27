// Package exitcode defines the process exit codes used by PortLens so that
// shell scripts can branch on outcomes. Documented in docs/exit-codes.md.
package exitcode

const (
	// Success indicates the command completed successfully.
	Success = 0
	// GeneralError is a catch-all for unexpected failures.
	GeneralError = 1
	// InvalidArguments indicates malformed flags or a bad port value.
	InvalidArguments = 2
	// PortNotFound indicates nothing is listening on the requested port.
	PortNotFound = 3
	// PermissionDenied indicates the operation requires privileges PortLens
	// will not attempt to obtain.
	PermissionDenied = 4
	// ProcessActionFailed indicates a process action (kill/restart) failed.
	ProcessActionFailed = 5
)
