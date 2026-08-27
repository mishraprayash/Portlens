// Package detect infers project and technology metadata from the filesystem
// and process command lines. Everything returned here is best-effort inference;
// callers are responsible for labeling it as such rather than presenting it as
// fact (see the model's Facts vs Inferences separation).
package detect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/portlens/portlens/internal/model"
)

// ProjectDetector discovers project metadata given a working directory.
type ProjectDetector interface {
	Detect(ctx context.Context, cwd string) *model.ProjectInfo
}

// NewProjectDetector returns the default filesystem-backed detector.
func NewProjectDetector() ProjectDetector { return fsProjectDetector{} }

type fsProjectDetector struct{}

type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// Detect walks up from cwd looking for the nearest project root (the first
// directory containing a recognized marker file).
func (fsProjectDetector) Detect(_ context.Context, cwd string) *model.ProjectInfo {
	if cwd == "" {
		return nil
	}
	dir := filepath.Clean(cwd)
	for {
		if info := scanProjectDir(dir); info != nil && info.Detected {
			return info
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// scanProjectDir inspects a single directory for project markers.
func scanProjectDir(dir string) *model.ProjectInfo {
	info := &model.ProjectInfo{Directory: dir}

	if exists(filepath.Join(dir, "package.json")) {
		info.Detected = true
		info.Runtime = "node"
		if pkg, err := readPackageJSON(filepath.Join(dir, "package.json")); err == nil {
			if pkg.Name != "" {
				info.Name = pkg.Name
			}
			info.Framework = frameworkFromPackageJSON(pkg)
			info.Language = "javascript"
		}
		info.PackageManager = packageManagerFromLocks(dir)
	} else if exists(filepath.Join(dir, "go.mod")) {
		info.Detected = true
		info.Runtime = "go"
		info.Language = "go"
		info.Name = moduleNameFromGoMod(filepath.Join(dir, "go.mod"))
	} else if exists(filepath.Join(dir, "Cargo.toml")) {
		info.Detected = true
		info.Runtime = "rust"
		info.Language = "rust"
		info.Name = filepath.Base(dir)
		info.PackageManager = "cargo"
	} else if exists(filepath.Join(dir, "pyproject.toml")) ||
		exists(filepath.Join(dir, "requirements.txt")) ||
		exists(filepath.Join(dir, "setup.py")) {
		info.Detected = true
		info.Runtime = "python"
		info.Language = "python"
		info.Name = filepath.Base(dir)
		info.PackageManager = packageManagerFromLocks(dir)
	} else if exists(filepath.Join(dir, "pom.xml")) {
		info.Detected = true
		info.Runtime = "java"
		info.Language = "java"
		info.Name = filepath.Base(dir)
		info.PackageManager = "maven"
	} else if exists(filepath.Join(dir, "build.gradle")) || exists(filepath.Join(dir, "build.gradle.kts")) {
		info.Detected = true
		info.Runtime = "java"
		info.Language = "java"
		info.Name = filepath.Base(dir)
		info.PackageManager = "gradle"
	}

	// Git repository detection (works even without a language marker).
	if repo, branch := gitInfo(dir); repo != "" {
		info.GitRepo = repo
		info.GitBranch = branch
		if !info.Detected {
			info.Detected = true
			info.Name = repo
		}
		if info.Name == "" {
			info.Name = repo
		}
	}

	if !info.Detected {
		return nil
	}
	return info
}

func gitInfo(dir string) (repo string, branch string) {
	gitDir := filepath.Join(dir, ".git")
	if !exists(gitDir) {
		return "", ""
	}
	if head, err := os.ReadFile(filepath.Join(gitDir, "HEAD")); err == nil {
		headStr := strings.TrimSpace(string(head))
		if strings.HasPrefix(headStr, "ref:") {
			parts := strings.Split(headStr, "/")
			branch = parts[len(parts)-1]
		}
	}
	// Prefer the directory name, then the origin remote.
	if cfg, err := os.ReadFile(filepath.Join(gitDir, "config")); err == nil {
		for _, line := range strings.Split(string(cfg), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "url = ") {
				u := strings.TrimSuffix(strings.TrimPrefix(line, "url = "), ".git")
				parts := strings.Split(strings.Trim(u, "/"), "/")
				repo = parts[len(parts)-1]
				break
			}
		}
	}
	if repo == "" {
		repo = filepath.Base(filepath.Dir(gitDir))
	}
	return repo, branch
}

func readPackageJSON(path string) (packageJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return packageJSON{}, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return packageJSON{}, err
	}
	return pkg, nil
}

// frameworkFromPackageJSON infers a framework from a Node project's manifest.
// The order matters: more specific/signature frameworks win.
func frameworkFromPackageJSON(pkg packageJSON) string {
	deps := map[string]bool{}
	for k := range pkg.Dependencies {
		deps[strings.ToLower(k)] = true
	}
	for k := range pkg.DevDependencies {
		deps[strings.ToLower(k)] = true
	}
	switch {
	case deps["@nestjs/core"] || deps["@nestjs/common"]:
		return "nestjs"
	case deps["next"]:
		return "next.js"
	case deps["nuxt"]:
		return "nuxt"
	case deps["@sveltejs/kit"]:
		return "sveltekit"
	case deps["fastify"]:
		return "fastify"
	case deps["express"]:
		return "express"
	case deps["koa"]:
		return "koa"
	case deps["@angular/core"]:
		return "angular"
	case deps["react"]:
		return "react"
	case deps["vue"]:
		return "vue"
	case deps["prisma"] || deps["@prisma/client"]:
		return "prisma"
	}
	return ""
}

func packageManagerFromLocks(dir string) string {
	switch {
	case exists(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm"
	case exists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	case exists(filepath.Join(dir, "package-lock.json")):
		return "npm"
	case exists(filepath.Join(dir, "Pipfile")):
		return "pipenv"
	case exists(filepath.Join(dir, "poetry.lock")):
		return "poetry"
	case exists(filepath.Join(dir, "Cargo.lock")):
		return "cargo"
	}
	return ""
}

func moduleNameFromGoMod(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			parts := strings.Split(fields[1], "/")
			return parts[len(parts)-1]
		}
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
