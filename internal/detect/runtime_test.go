package detect

import (
	"testing"

	"github.com/portlens/portlens/internal/model"
)

func TestDetectRuntime(t *testing.T) {
	cases := []struct {
		p    *model.ProcessInfo
		want string
	}{
		{&model.ProcessInfo{Name: "node"}, "node"},
		{&model.ProcessInfo{Name: "python3.11"}, "python"},
		{&model.ProcessInfo{Name: "java"}, "java"},
		{&model.ProcessInfo{Name: "redis-server"}, "redis"},
		{&model.ProcessInfo{Name: "postgres"}, "postgres"},
		{&model.ProcessInfo{Name: "docker"}, "docker"},
		{&model.ProcessInfo{Name: "go", Command: "go run main.go"}, "go"},
		{&model.ProcessInfo{Name: "totally-unknown"}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := DetectRuntime(c.p); got != c.want {
			t.Errorf("DetectRuntime(%v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestDetectFramework(t *testing.T) {
	p := &model.ProcessInfo{Name: "node", Command: "nest start"}
	if got := DetectFramework(p, nil); got != "nestjs" {
		t.Errorf("DetectFramework(nest start) = %q, want nestjs", got)
	}

	proj := &model.ProjectInfo{Framework: "express"}
	if got := DetectFramework(&model.ProcessInfo{Name: "node", Command: "node server.js"}, proj); got != "express" {
		t.Errorf("DetectFramework(express project) = %q, want express", got)
	}
}

func TestFrameworkDisplay(t *testing.T) {
	cases := map[string]string{
		"nestjs":  "NestJS",
		"next.js": "Next.js",
		"express": "Express",
		"":        "",
	}
	for in, want := range cases {
		if got := FrameworkDisplay(in); got != want {
			t.Errorf("FrameworkDisplay(%q) = %q, want %q", in, got, want)
		}
	}
}
