package actions

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/mishraprayash/Portlens/internal/platform"
)

func TestTerminateTreeRefusesProtectedPIDs(t *testing.T) {
	for _, pid := range []int32{-1, 0, 1, int32(os.Getpid())} {
		ctrl := &stubController{alive: map[int32]bool{pid: true}}
		mgr := NewManager(&platform.Platform{Controller: ctrl}, &bytes.Buffer{}, nil)
		err := mgr.TerminateTree(context.Background(), pid, true)
		if !errors.Is(err, ErrProtectedProcess) {
			t.Errorf("pid %d: err = %v, want ErrProtectedProcess", pid, err)
		}
		if len(ctrl.signaled) != 0 {
			t.Errorf("pid %d: signals sent: %v", pid, ctrl.signaled)
		}
	}
}
