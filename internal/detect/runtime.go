package detect

import (
	"strings"

	"github.com/portlens/portlens/internal/model"
)

// runtimeSignatures maps a process name/command substring to a canonical
// runtime label. Order is significant: more specific names come first.
var runtimeSignatures = []struct {
	name    string
	matches func(p *model.ProcessInfo) bool
}{
	{"node", func(p *model.ProcessInfo) bool { return nameEq(p, "node") || hasExe(p, "node") }},
	{"bun", func(p *model.ProcessInfo) bool { return nameEq(p, "bun") }},
	{"deno", func(p *model.ProcessInfo) bool { return nameEq(p, "deno") }},
	{"python", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "python") }},
	{"go", func(p *model.ProcessInfo) bool {
		return nameEq(p, "go") || hasCmd(p, "go run") || hasCmd(p, "go build") || hasExe(p, "go-build")
	}},
	{"java", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "java") }},
	{"ruby", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "ruby") }},
	{"php", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "php") }},
	{"rust", func(p *model.ProcessInfo) bool { return nameEq(p, "cargo") || hasExe(p, "cargo") }},
	{"docker", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "docker") || nameHasPrefix(p, "containerd") }},
	{"postgres", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "postgres") }},
	{"redis", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "redis") }},
	{"mongodb", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "mongod") }},
	{"mysql", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "mysqld") || nameHasPrefix(p, "mariadbd") }},
	{"nginx", func(p *model.ProcessInfo) bool { return nameEq(p, "nginx") }},
	{"elixir", func(p *model.ProcessInfo) bool { return nameHasPrefix(p, "beam.smp") || nameEq(p, "elixir") }},
}

// DetectRuntime infers the runtime from a process's name, executable path, and
// command line. It returns "" when nothing recognizable is found.
func DetectRuntime(p *model.ProcessInfo) string {
	if p == nil {
		return ""
	}
	for _, sig := range runtimeSignatures {
		if sig.matches(p) {
			return sig.name
		}
	}
	return ""
}

// DetectFramework augments runtime detection with framework hints gleaned from
// the process command line. It returns "" when no framework can be inferred.
func DetectFramework(p *model.ProcessInfo, proj *model.ProjectInfo) string {
	if p == nil {
		return ""
	}
	cmd := strings.ToLower(p.Command)
	joined := cmd
	if proj != nil {
		joined += " " + strings.ToLower(proj.Framework)
	}
	switch {
	case containsAny(cmd, "nest start", "nest.js", "@nestjs"):
		return "nestjs"
	case containsAny(joined, "next"):
		return "next.js"
	case containsAny(cmd, "vite"):
		return "vite"
	case containsAny(cmd, "ts-node"):
		return "ts-node"
	case containsAny(cmd, "uvicorn"):
		return "uvicorn"
	case containsAny(cmd, "gunicorn"):
		return "gunicorn"
	case containsAny(cmd, "flask"):
		return "flask"
	case containsAny(cmd, "django"):
		return "django"
	case containsAny(cmd, "spring", "spring-boot"):
		return "spring"
	case containsAny(cmd, "rails"):
		return "rails"
	case containsAny(cmd, "prisma"):
		return "prisma"
	case containsAny(cmd, "express"):
		return "express"
	case containsAny(cmd, "fastify"):
		return "fastify"
	}
	if proj != nil && proj.Framework != "" {
		return proj.Framework
	}
	return ""
}

func nameEq(p *model.ProcessInfo, name string) bool {
	return strings.EqualFold(p.Name, name)
}

func nameHasPrefix(p *model.ProcessInfo, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(p.Name), prefix)
}

func hasExe(p *model.ProcessInfo, sub string) bool {
	return strings.Contains(strings.ToLower(p.Exe), sub)
}

func hasCmd(p *model.ProcessInfo, sub string) bool {
	return containsAny(strings.ToLower(p.Command), sub)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ShortRuntimeName returns a compact display label for a runtime.
func ShortRuntimeName(runtime string) string {
	switch runtime {
	case "node":
		return "Node.js"
	case "python":
		return "Python"
	case "go":
		return "Go"
	case "java":
		return "Java"
	case "ruby":
		return "Ruby"
	case "php":
		return "PHP"
	case "rust":
		return "Rust"
	case "docker":
		return "Docker"
	case "postgres":
		return "PostgreSQL"
	case "redis":
		return "Redis"
	case "mongodb":
		return "MongoDB"
	case "mysql":
		return "MySQL"
	case "nginx":
		return "Nginx"
	case "elixir":
		return "Elixir"
	case "bun":
		return "Bun"
	case "deno":
		return "Deno"
	}
	return ""
}

// FrameworkDisplay maps an internal framework identifier to its canonical
// display name.
func FrameworkDisplay(name string) string {
	switch strings.ToLower(name) {
	case "nestjs":
		return "NestJS"
	case "next.js", "next":
		return "Next.js"
	case "fastify":
		return "Fastify"
	case "express":
		return "Express"
	case "koa":
		return "Koa"
	case "sveltekit":
		return "SvelteKit"
	case "nuxt":
		return "Nuxt"
	case "angular":
		return "Angular"
	case "react":
		return "React"
	case "vue":
		return "Vue"
	case "prisma":
		return "Prisma"
	case "vite":
		return "Vite"
	case "ts-node":
		return "ts-node"
	case "uvicorn":
		return "Uvicorn"
	case "gunicorn":
		return "Gunicorn"
	case "flask":
		return "Flask"
	case "django":
		return "Django"
	case "spring":
		return "Spring"
	case "rails":
		return "Rails"
	}
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
