//go:build linux

package platform

import (
	"context"
	"os"
	"testing"
)

func TestParseStatRow(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		want   string
		ppid   int32
		zombie bool
	}{
		{"simple", "1234 (cat) S 99 99 99 0 -1 4194304 0 0 0 0 0 0 0 0 0", "cat", 99, false},
		{"name with spaces", "5 (a b c) S 1 1 1 0 -1 4194304 0 0 0 0 0 0 0 0 0", "a b c", 1, false},
		{"zombie", "7 (defunct) Z 2 2 2 0 -1 4194304 0 0 0 0 0 0 0 0 0", "defunct", 2, true},
	}
	for _, c := range cases {
		row, ok := parseStatRow([]byte(c.data))
		if !ok {
			t.Errorf("%s: parse failed", c.name)
			continue
		}
		if row.name != c.want {
			t.Errorf("%s: name = %q, want %q", c.name, row.name, c.want)
		}
		if row.ppid != c.ppid {
			t.Errorf("%s: ppid = %d, want %d", c.name, row.ppid, c.ppid)
		}
		if row.zombie != c.zombie {
			t.Errorf("%s: zombie = %v, want %v", c.name, row.zombie, c.zombie)
		}
	}
}

func TestParseStatRowMalformed(t *testing.T) {
	if _, ok := parseStatRow([]byte("no parens here")); ok {
		t.Error("expected parse failure")
	}
	if _, ok := parseStatRow([]byte("1234 (x")); ok {
		t.Error("expected parse failure for unterminated comm")
	}
}

func TestLinuxProcInfoSelf(t *testing.T) {
	insp := linuxProcessInspector{}
	info, err := insp.Info(context.Background(), int32(os.Getpid()))
	if err != nil {
		t.Fatalf("Info(self): %v", err)
	}
	if info.PID != int32(os.Getpid()) || info.Name == "" {
		t.Errorf("info = %+v", info)
	}
	if info.PPID <= 0 {
		t.Errorf("ppid = %d, want > 0", info.PPID)
	}
	if info.StartTime.IsZero() {
		t.Error("start time should be set for full Info")
	}
	if len(info.Cmdline) == 0 {
		t.Error("cmdline should be non-empty for the test process")
	}
}

func TestLinuxProcInfoBasicSkipsHeavyFields(t *testing.T) {
	insp := linuxProcessInspector{}
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
	full, err := insp.Info(context.Background(), int32(os.Getpid()))
	if err != nil {
		t.Fatalf("Info(self): %v", err)
	}
	if full.StartTime.IsZero() {
		t.Error("full Info should populate start time")
	}
}

func TestLinuxProcessTable(t *testing.T) {
	rows, err := loadProcessTable()
	if err != nil {
		t.Fatalf("loadProcessTable: %v", err)
	}
	if len(rows) < 1 {
		t.Fatal("expected at least one process row")
	}
	// The test process itself must be present, with a resolvable parent.
	found := false
	for _, r := range rows {
		if r.pid == int32(os.Getpid()) {
			found = true
			if r.ppid <= 0 {
				t.Errorf("own ppid = %d, want > 0", r.ppid)
			}
			break
		}
	}
	if !found {
		t.Error("own pid missing from process table")
	}
}

func TestIsProcessAliveLinux(t *testing.T) {
	if !isProcessAlive(int32(os.Getpid())) {
		t.Error("own process should be alive")
	}
	if isProcessAlive(99999999) {
		t.Error("nonexistent pid should not be alive")
	}
}
