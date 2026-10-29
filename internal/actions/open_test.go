package actions

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
	"github.com/mishraprayash/Portlens/internal/platform"
)

func TestLocalURL(t *testing.T) {
	cases := []struct {
		report *model.Report
		want   string
	}{
		{&model.Report{Port: 3000, Address: "127.0.0.1"}, "http://localhost:3000"},
		{&model.Report{Port: 3000, Address: "0.0.0.0"}, "http://localhost:3000"},
		{&model.Report{Port: 8080, Address: "::1"}, "http://localhost:8080"},
		{&model.Report{Port: 8080, Address: "::"}, "http://localhost:8080"},
		{&model.Report{Port: 8080, Address: "192.168.1.10"}, "http://192.168.1.10:8080"},
	}
	for _, c := range cases {
		if got := LocalURL(c.report); got != c.want {
			t.Errorf("LocalURL(%q) = %q, want %q", c.report.Address, got, c.want)
		}
	}
}

func TestOpenDelegatesToPlatformOpener(t *testing.T) {
	var got string
	p := &platform.Platform{OpenURL: func(_ context.Context, u string) error {
		got = u
		return nil
	}}
	var out bytes.Buffer
	m := &Manager{Platform: p, Out: &out, Wait: time.Second}
	rep := &model.Report{Port: 3000, Address: "127.0.0.1", Protocol: model.ProtocolTCP}

	if err := m.Open(context.Background(), rep); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != "http://localhost:3000" {
		t.Errorf("opener received %q, want http://localhost:3000", got)
	}
	if !strings.Contains(out.String(), "Opening http://localhost:3000") {
		t.Errorf("output %q does not announce the URL", out.String())
	}
}
