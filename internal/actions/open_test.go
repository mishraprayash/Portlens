package actions

import (
	"testing"

	"github.com/portlens/portlens/internal/model"
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
