package detect

import (
	"testing"

	"github.com/portlens/portlens/internal/model"
)

func TestLookupService(t *testing.T) {
	cases := []struct {
		port uint16
		want string
	}{
		{22, "SSH"},
		{80, "HTTP"},
		{88, "Kerberos"},
		{443, "HTTPS"},
		{5353, "mDNS (DNS-SD)"},
		{5432, "PostgreSQL"},
		{5900, "Screen Sharing (VNC)"},
		{6379, "Redis"},
		{27017, "MongoDB"},
		{1, ""},
		{52381, ""},
	}
	for _, c := range cases {
		if got := LookupService(c.port); got != c.want {
			t.Errorf("LookupService(%d) = %q, want %q", c.port, got, c.want)
		}
	}
}

func TestProcessOrigin(t *testing.T) {
	cases := []struct {
		name string
		p    *model.ProcessInfo
		want model.Origin
	}{
		{"nil", nil, ""},
		{"system path", &model.ProcessInfo{Exe: "/System/Library/PrivateFrameworks/Heimdal.framework/Helpers/kdc"}, model.OriginSystem},
		{"system usr sbin", &model.ProcessInfo{Exe: "/usr/sbin/mDNSResponder"}, model.OriginSystem},
		{"system libexec", &model.ProcessInfo{Exe: "/usr/libexec/rapportd"}, model.OriginSystem},
		{"kernel", &model.ProcessInfo{Name: "kernel_task"}, model.OriginSystem},
		{"launchd name", &model.ProcessInfo{Name: "launchd"}, model.OriginSystem},
		{"brew", &model.ProcessInfo{Exe: "/opt/homebrew/opt/postgresql@16/bin/postgres"}, model.OriginUser},
		{"usr local brew", &model.ProcessInfo{Exe: "/usr/local/Cellar/redis/7/bin/redis-server"}, model.OriginUser},
		{"app bundle", &model.ProcessInfo{Exe: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}, model.OriginUser},
		{"user home", &model.ProcessInfo{Exe: "/Users/prayash/.nvm/versions/node/v22/bin/node"}, model.OriginUser},
		{"user daemon name no exe", &model.ProcessInfo{Name: "postgres"}, model.OriginUser},
		{"redis name no exe", &model.ProcessInfo{Name: "redis-server"}, model.OriginUser},
		{"node name no exe", &model.ProcessInfo{Name: "node"}, model.OriginUser},
		{"unknown", &model.ProcessInfo{Name: "my-custom-thing"}, ""},
	}
	for _, c := range cases {
		if got := ProcessOrigin(c.p); got != c.want {
			t.Errorf("ProcessOrigin(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}
