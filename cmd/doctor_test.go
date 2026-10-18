package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func fakeEnv() doctorEnv {
	return doctorEnv{
		goos: "linux", goarch: "amd64", euid: 1000,
		lookPath:    func(s string) (string, error) { return "/usr/bin/" + s, nil },
		listeners:   func(context.Context) (int, int, error) { return 5, 0, nil },
		dialSocket:  func(string) error { return nil },
		configCheck: func() (string, error) { return "/cfg.json", nil },
	}
}

func statusOf(checks []doctorCheck, name string) string {
	for _, c := range checks {
		if c.Name == name {
			return c.Status
		}
	}
	return "missing"
}

func TestDoctorAllHealthy(t *testing.T) {
	checks := runDoctorChecks(context.Background(), fakeEnv())
	for _, c := range checks {
		if c.Status != checkOK && c.Status != checkSkip {
			t.Errorf("%s = %s (%s)", c.Name, c.Status, c.Detail)
		}
	}
	if doctorExitCode(checks) != 0 {
		t.Error("healthy env should exit 0")
	}
}

func TestDoctorDegradedAndFailing(t *testing.T) {
	env := fakeEnv()
	env.lookPath = func(string) (string, error) { return "", errors.New("nope") }
	env.listeners = func(context.Context) (int, int, error) { return 4, 2, nil }
	env.euid = 0
	checks := runDoctorChecks(context.Background(), env)
	for name, want := range map[string]string{
		"clipboard": checkWarn, "notifications": checkWarn,
		"listing ports": checkWarn, "privileges": checkWarn,
	} {
		if got := statusOf(checks, name); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	if doctorExitCode(checks) != 0 {
		t.Error("warnings must not fail the run")
	}

	env.listeners = func(context.Context) (int, int, error) { return 0, 0, errors.New("boom") }
	env.configCheck = func() (string, error) { return "/cfg.json", errors.New("corrupt") }
	checks = runDoctorChecks(context.Background(), env)
	if statusOf(checks, "listing ports") != checkFail || statusOf(checks, "config") != checkFail {
		t.Errorf("expected failures: %+v", checks)
	}
	if doctorExitCode(checks) != 1 {
		t.Error("failed core check should exit 1")
	}
}

func TestDoctorDarwinMissingLsofIsFatal(t *testing.T) {
	env := fakeEnv()
	env.goos = "darwin"
	env.lookPath = func(s string) (string, error) {
		if s == "lsof" {
			return "", errors.New("missing")
		}
		return "/bin/" + s, nil
	}
	if statusOf(runDoctorChecks(context.Background(), env), "port source") != checkFail {
		t.Error("missing lsof must fail on darwin")
	}
}

func TestDoctorContainerSocket(t *testing.T) {
	env := fakeEnv()
	env.sockets = []string{t.TempDir()} // exists, so Stat passes
	env.dialSocket = func(string) error { return errors.New("refused") }
	if got := statusOf(runDoctorChecks(context.Background(), env), "container runtime"); got != checkWarn {
		t.Errorf("unreachable socket = %s, want warn", got)
	}
	env.sockets = []string{"/definitely/not/here.sock"}
	if got := statusOf(runDoctorChecks(context.Background(), env), "container runtime"); got != checkSkip {
		t.Errorf("no socket = %s, want skip", got)
	}
}

func TestRunDoctorOutput(t *testing.T) {
	var buf bytes.Buffer
	if code := runDoctor(context.Background(), &buf, false, false, fakeEnv()); code != 0 {
		t.Fatalf("code %d", code)
	}
	if strings.Contains(buf.String(), "\x1b[") || !strings.Contains(buf.String(), "platform") {
		t.Errorf("bad text output: %q", buf.String())
	}
	buf.Reset()
	runDoctor(context.Background(), &buf, true, false, fakeEnv())
	var parsed []doctorCheck
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil || len(parsed) == 0 {
		t.Fatalf("invalid JSON: %v %q", err, buf.String())
	}
}
