package platform

import (
	"testing"

	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/model"
)

// EPERM from signal(2) must surface as exit code 4 as documented in
// docs/exit-codes.md; it only does if both packages share one sentinel.
func TestErrPermissionDeniedMapsToPermissionDenied(t *testing.T) {
	if ErrPermissionDenied != model.ErrPermissionDenied {
		t.Fatal("platform and model permission sentinels must share one identity")
	}
	if got := model.MapExitCode(ErrPermissionDenied); got != exitcode.PermissionDenied {
		t.Errorf("MapExitCode(ErrPermissionDenied) = %d, want %d", got, exitcode.PermissionDenied)
	}
}
