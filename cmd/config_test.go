package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mishraprayash/Portlens/internal/exitcode"
)

func TestNewConfirmYesBypassesStdin(t *testing.T) {
	var out bytes.Buffer
	confirm := newConfirm(&out, strings.NewReader("n\n"), true)
	ok, err := confirm("prompt?")
	if err != nil || !ok {
		t.Errorf("confirm(--yes) = %v, %v; want true, nil", ok, err)
	}
	if out.Len() != 0 {
		t.Errorf("confirm(--yes) wrote %q, want no prompt", out.String())
	}
}

func TestNewConfirmRefusesNonTerminal(t *testing.T) {
	var out bytes.Buffer
	confirm := newConfirm(&out, strings.NewReader("y\n"), false)
	ok, err := confirm("prompt?")
	if ok {
		t.Error("confirm(non-tty) = true, want false")
	}
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("confirm(non-tty) error = %v, want it to mention --yes", err)
	}
}

func TestIsTerminalReaderFalseForBuffers(t *testing.T) {
	if isTerminalReader(strings.NewReader("x")) {
		t.Error("isTerminalReader(strings.Reader) = true, want false")
	}
	if isTerminalReader(&bytes.Buffer{}) {
		t.Error("isTerminalReader(bytes.Buffer) = true, want false")
	}
}

func TestRunConfigDispatch(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	if code := runConfig(nil, &stdout, &stderr); code != exitcode.Success {
		t.Errorf("runConfig(nil) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "portlens config") {
		t.Errorf("usage output = %q, want config usage", stdout.String())
	}

	stdout.Reset()
	if code := runConfig([]string{"bogus"}, &stdout, &stderr); code != exitcode.InvalidArguments {
		t.Errorf("runConfig(bogus) = %d, want %d", code, exitcode.InvalidArguments)
	}
	if !strings.Contains(stderr.String(), `unknown subcommand "bogus"`) {
		t.Errorf("stderr = %q, want unknown subcommand error", stderr.String())
	}

	stdout.Reset()
	if code := runConfig([]string{"path"}, &stdout, &stderr); code != exitcode.Success {
		t.Errorf("runConfig(path) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "config.json") {
		t.Errorf("path output = %q, want config.json", stdout.String())
	}
}

func TestConfigLifecycle(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer

	if code := configList(&stdout, &stderr); code != exitcode.Success {
		t.Fatalf("configList(empty) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "No port groups configured.") {
		t.Errorf("empty list output = %q", stdout.String())
	}

	stdout.Reset()
	if code := configAdd([]string{"dev", "3000", "3002-3004"}, &stdout, &stderr); code != exitcode.Success {
		t.Fatalf("configAdd = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Saved group @dev: 3000, 3002-3004") {
		t.Errorf("add output = %q, want saved group summary", stdout.String())
	}

	stdout.Reset()
	if code := configList(&stdout, &stderr); code != exitcode.Success {
		t.Fatalf("configList = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "@dev") {
		t.Errorf("list output = %q, want @dev", stdout.String())
	}

	stdout.Reset()
	if code := configShow([]string{"dev"}, &stdout, &stderr); code != exitcode.Success {
		t.Fatalf("configShow = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "3000, 3002-3004" {
		t.Errorf("show output = %q, want port list", got)
	}

	stdout.Reset()
	if code := configRemove([]string{"dev"}, &stdout, &stderr); code != exitcode.Success {
		t.Fatalf("configRemove = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Removed group @dev") {
		t.Errorf("remove output = %q", stdout.String())
	}

	stdout.Reset()
	if code := configShow([]string{"dev"}, &stdout, &stderr); code != exitcode.InvalidArguments {
		t.Errorf("configShow(removed) = %d, want %d", code, exitcode.InvalidArguments)
	}
}

func TestConfigUsageErrors(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer

	if code := configShow(nil, &stdout, &stderr); code != exitcode.InvalidArguments {
		t.Errorf("configShow(no args) = %d, want %d", code, exitcode.InvalidArguments)
	}
	if code := configAdd([]string{"onlyname"}, &stdout, &stderr); code != exitcode.InvalidArguments {
		t.Errorf("configAdd(no ports) = %d, want %d", code, exitcode.InvalidArguments)
	}
	if code := configAdd([]string{"g", "notaport"}, &stdout, &stderr); code != exitcode.InvalidArguments {
		t.Errorf("configAdd(bad port) = %d, want %d", code, exitcode.InvalidArguments)
	}
	if code := configRemove([]string{"missing"}, &stdout, &stderr); code != exitcode.InvalidArguments {
		t.Errorf("configRemove(missing) = %d, want %d", code, exitcode.InvalidArguments)
	}
	if !strings.Contains(stderr.String(), `group "missing" not found`) {
		t.Errorf("stderr = %q, want group not found", stderr.String())
	}
}

func TestConfigGroupLookup(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := configAdd([]string{"web", "80", "443"}, &stdout, &stderr); code != exitcode.Success {
		t.Fatalf("configAdd = %d", code)
	}
	ports, err := configGroupLookup("web")
	if err != nil {
		t.Fatalf("configGroupLookup(web) = %v", err)
	}
	if len(ports) != 2 {
		t.Errorf("ports = %v, want 2 entries", ports)
	}
	if _, err := configGroupLookup("missing"); err == nil {
		t.Error("configGroupLookup(missing) = nil error, want not found")
	}
}
