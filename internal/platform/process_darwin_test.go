//go:build darwin

package platform

import (
	"context"
	"os"
	"testing"
)

func TestDarwinProcInfoSelf(t *testing.T) {
	insp := darwinProcessInspector{}
	pid := int32(os.Getpid())
	info, err := insp.Info(context.Background(), pid)
	if err != nil {
		t.Fatalf("Info(self): %v", err)
	}
	if info.PID != pid || info.Name == "" {
		t.Errorf("info = %+v", info)
	}
	if info.PPID <= 0 {
		t.Errorf("ppid = %d, want > 0", info.PPID)
	}
	if info.StartTime.IsZero() {
		t.Error("start time should be set for full Info")
	}
	if info.User == "" {
		t.Error("user should be set for full Info")
	}
	if len(info.Cmdline) == 0 {
		t.Error("cmdline should be non-empty for the test process")
	}
	if info.Exe == "" {
		t.Error("exe should be resolved")
	}
	if info.CWD == "" {
		t.Error("cwd should be resolved via libproc")
	}
}

func TestDarwinProcInfoBasicSkipsHeavyFields(t *testing.T) {
	insp := darwinProcessInspector{}
	basic, err := insp.InfoBasic(context.Background(), int32(os.Getpid()))
	if err != nil {
		t.Fatalf("InfoBasic(self): %v", err)
	}
	if !basic.StartTime.IsZero() {
		t.Error("InfoBasic should not populate start time")
	}
	if basic.User != "" {
		t.Error("InfoBasic should not populate user")
	}
}

func TestDarwinProcInfoBasicBatch(t *testing.T) {
	insp := darwinProcessInspector{}
	pids := []int32{int32(os.Getpid()), 1, 99999999}
	res, err := insp.InfoBasicBatch(context.Background(), pids)
	if err != nil {
		t.Fatalf("InfoBasicBatch failed: %v", err)
	}
	if self, ok := res[int32(os.Getpid())]; !ok || self.Name == "" {
		t.Errorf("expected info for self PID, got %+v", self)
	}
	if _, ok := res[99999999]; ok {
		t.Error("nonexistent PID should be omitted from batch result")
	}
}

func TestDarwinArgs(t *testing.T) {
	exe, argv, err := darwinArgs(int32(os.Getpid()))
	if err != nil {
		t.Fatalf("darwinArgs: %v", err)
	}
	if exe == "" {
		t.Error("exec path should be non-empty")
	}
	if len(argv) == 0 {
		t.Error("argv should be non-empty for the test process")
	}
}

func TestDarwinCwd(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got := darwinCwd(int32(os.Getpid()))
	if got == "" {
		t.Fatal("darwinCwd returned empty")
	}
	if got != wd {
		t.Errorf("cwd = %q, want %q", got, wd)
	}
}

func TestIsProcessAliveDarwin(t *testing.T) {
	if !isProcessAlive(int32(os.Getpid())) {
		t.Error("own process should be alive")
	}
	if isProcessAlive(99999999) {
		t.Error("nonexistent pid should not be alive")
	}
}
