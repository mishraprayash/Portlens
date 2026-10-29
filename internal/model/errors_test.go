package model

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

	"github.com/mishraprayash/Portlens/internal/exitcode"
)

func TestMapExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, exitcode.Success},
		{"port not found", ErrPortNotFound, exitcode.PortNotFound},
		{"permission denied", ErrPermissionDenied, exitcode.PermissionDenied},
		{
			"permission denied wrapped",
			fmt.Errorf("kill failed: %w", ErrPermissionDenied),
			exitcode.PermissionDenied,
		},
		{
			"permission denied from os",
			&os.PathError{Op: "open", Path: "/etc/shadow", Err: syscall.EACCES},
			exitcode.PermissionDenied,
		},
		{"process action failed", ErrProcessActionFailed, exitcode.ProcessActionFailed},
		{"invalid port", ErrInvalidPort, exitcode.InvalidArguments},
		{"invalid arguments", ErrInvalidArguments, exitcode.InvalidArguments},
		{"unknown error", errors.New("boom"), exitcode.GeneralError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MapExitCode(c.err); got != c.want {
				t.Errorf("MapExitCode(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}
