package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/platform"
)

// LocalURL builds the most appropriate HTTP URL for a listener. Wildcard and
// loopback addresses are normalized to "localhost"; specific interface
// addresses are used verbatim.
func LocalURL(report *model.Report) string {
	host := report.Address
	switch host {
	case "0.0.0.0", "::", "*", "":
		host = "localhost"
	case "127.0.0.1", "::1":
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", joinHostPort(host, int(report.Port)), report.Port)
}

func joinHostPort(host string, port int) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

// Open opens the service in the default browser. It warns when the service is
// unlikely to be HTTP. Opening is non-destructive, so it does not prompt.
func (m *Manager) Open(ctx context.Context, report *model.Report) error {
	url := LocalURL(report)
	if !looksLikeHTTP(report) {
		fmt.Fprintf(m.Out, "Note: this service may not be HTTP; opening %s may fail.\n", url)
	}
	fmt.Fprintf(m.Out, "Opening %s\n", url)
	return platform.OpenURL(ctx, url)
}

// looksLikeHTTP makes a conservative guess about whether a process serves HTTP,
// based on known HTTP runtimes/frameworks or common dev-server command lines.
func looksLikeHTTP(report *model.Report) bool {
	if report.Project != nil {
		switch report.Project.Framework {
		case "nestjs", "express", "fastify", "next.js", "koa", "react", "vue",
			"angular", "sveltekit", "nuxt", "flask", "django", "rails":
			return true
		}
		switch report.Project.Runtime {
		case "node", "python", "ruby", "php", "go", "java", "rust":
			return true
		}
	}
	if report.Process != nil {
		cmd := strings.ToLower(report.Process.Command)
		for _, token := range []string{"dev", "serve", "server", "start", "http", "web"} {
			if strings.Contains(cmd, token) {
				return true
			}
		}
	}
	return false
}
