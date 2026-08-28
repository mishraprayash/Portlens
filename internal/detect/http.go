package detect

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/portlens/portlens/internal/model"
)

var titleTagRegex = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)

// ProbeHTTP sends a lightweight HTTP GET request to check for an active HTTP service,
// extracting status code, latency, Server header, and HTML <title>.
func ProbeHTTP(ctx context.Context, addr string, port uint16) *model.HTTPProbe {
	host := addr
	if host == "" || host == "0.0.0.0" || host == "::" || host == "*" || strings.HasPrefix(host, "127.") {
		host = "127.0.0.1"
	}

	targetURL := fmt.Sprintf("http://%s:%d/", host, port)

	probeCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "PortLens/1.0")
	req.Header.Set("Accept", "text/html,application/json,*/*")

	client := &http.Client{
		Timeout: 300 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	latency := time.Since(start)

	probe := &model.HTTPProbe{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Server:     resp.Header.Get("Server"),
		Latency:    latency,
	}

	// Read up to 4KB of body to extract title
	limitReader := io.LimitReader(resp.Body, 4096)
	body, _ := io.ReadAll(limitReader)
	if len(body) > 0 {
		if matches := titleTagRegex.FindSubmatch(body); len(matches) > 1 {
			probe.Title = strings.TrimSpace(string(matches[1]))
		}
	}

	return probe
}
