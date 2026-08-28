package detect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestProbeHTTPSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "Vite/5.0.0")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head><title>My Awesome Dashboard</title></head><body>Hello</body></html>"))
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(u.Port())

	probe := ProbeHTTP(context.Background(), u.Hostname(), uint16(port))
	if probe == nil {
		t.Fatal("expected non-nil HTTPProbe")
	}
	if probe.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", probe.StatusCode)
	}
	if probe.Server != "Vite/5.0.0" {
		t.Errorf("Server = %q, want Vite/5.0.0", probe.Server)
	}
	if probe.Title != "My Awesome Dashboard" {
		t.Errorf("Title = %q, want 'My Awesome Dashboard'", probe.Title)
	}
}

func TestProbeHTTPUnreachable(t *testing.T) {
	probe := ProbeHTTP(context.Background(), "127.0.0.1", 64999)
	if probe != nil {
		t.Errorf("expected nil probe for unreachable port, got %+v", probe)
	}
}
