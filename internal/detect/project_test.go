package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectNodeNestJS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"),
		`{"name":"orbit-backend","dependencies":{"@nestjs/core":"^10"}}`)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")

	info := (fsProjectDetector{}).Detect(context.Background(), dir)
	if info == nil || !info.Detected {
		t.Fatalf("expected detected project, got %+v", info)
	}
	if info.Runtime != "node" {
		t.Errorf("runtime = %q, want node", info.Runtime)
	}
	if info.Framework != "nestjs" {
		t.Errorf("framework = %q, want nestjs", info.Framework)
	}
	if info.Name != "orbit-backend" {
		t.Errorf("name = %q, want orbit-backend", info.Name)
	}
	if info.PackageManager != "pnpm" {
		t.Errorf("package manager = %q, want pnpm", info.PackageManager)
	}
}

func TestDetectGoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module github.com/acme/foo\n\ngo 1.21\n")

	info := (fsProjectDetector{}).Detect(context.Background(), dir)
	if info == nil || !info.Detected {
		t.Fatalf("expected detected project, got %+v", info)
	}
	if info.Runtime != "go" || info.Name != "foo" {
		t.Errorf("got runtime=%q name=%q, want go/foo", info.Runtime, info.Name)
	}
}

func TestDetectWalksUpDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"walkup"}`)
	sub := filepath.Join(dir, "src", "deep", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	info := (fsProjectDetector{}).Detect(context.Background(), sub)
	if info == nil || info.Name != "walkup" {
		t.Fatalf("expected to walk up to project root, got %+v", info)
	}
}

func TestDetectNoProject(t *testing.T) {
	dir := t.TempDir()
	info := (fsProjectDetector{}).Detect(context.Background(), dir)
	if info != nil {
		t.Fatalf("expected nil for empty dir, got %+v", info)
	}
}

func TestDetectGitInfo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(dir, ".git", "config"), "[remote \"origin\"]\n\turl = https://github.com/acme/orbit-backend.git\n")

	info := (fsProjectDetector{}).Detect(context.Background(), dir)
	if info == nil {
		t.Fatal("expected detected git project")
	}
	if info.GitRepo != "orbit-backend" {
		t.Errorf("git repo = %q, want orbit-backend", info.GitRepo)
	}
	if info.GitBranch != "main" {
		t.Errorf("git branch = %q, want main", info.GitBranch)
	}
}
