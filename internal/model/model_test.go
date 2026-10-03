package model

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{72 * time.Minute, "1h 12m"},
		{time.Hour, "1h 0m"},
		{0, "0s"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.d); got != c.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestProtocolNormalize(t *testing.T) {
	if ProtocolTCP6.Normalize() != ProtocolTCP {
		t.Errorf("tcp6 should normalize to tcp")
	}
	if ProtocolUDP6.Normalize() != ProtocolUDP {
		t.Errorf("udp6 should normalize to udp")
	}
}

func TestListenerKey(t *testing.T) {
	l := Listener{Protocol: ProtocolTCP, Port: 3000}
	if l.Key() != "tcp:3000" {
		t.Errorf("key = %q, want tcp:3000", l.Key())
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		b    uint64
		want string
	}{
		{500, "500 B"},
		{1024, "1 KB"},
		{1024 * 50, "50 KB"},
		{1024 * 1024 * 128, "128 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{uint64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.b); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.b, got, c.want)
		}
	}
}

func TestFormatAddr(t *testing.T) {
	cases := []struct {
		addr string
		port uint16
		want string
	}{
		{"127.0.0.1", 8080, "127.0.0.1:8080"},
		{"0.0.0.0", 80, "0.0.0.0:80"},
		{"::1", 5432, "[::1]:5432"},
		{"::", 3000, "[::]:3000"},
		{"fe80::1%lo0", 443, "[fe80::1%lo0]:443"},
	}
	for _, c := range cases {
		if got := FormatAddr(c.addr, c.port); got != c.want {
			t.Errorf("FormatAddr(%q, %d) = %q, want %q", c.addr, c.port, got, c.want)
		}
	}
}

func TestPortEntryMatches(t *testing.T) {
	entry := PortEntry{
		Port:     6379,
		Protocol: ProtocolTCP,
		Process:  "redis-server",
		Service:  "redis",
		Project:  "cache",
		Runtime:  "docker",
		Address:  "127.0.0.1",
		Status:   "listening",
		Origin:   OriginUser,
		Container: &Container{
			Name:           "cache-1",
			Image:          "redis:7-alpine",
			ComposeProject: "shop",
			ComposeService: "database",
		},
	}
	cases := []struct {
		name string
		q    string
		want bool
	}{
		{"empty matches", "", true},
		{"whitespace only", "   ", true},
		{"port", "6379", true},
		{"process", "redis-server", true},
		{"process case-insensitive", "REDIS", true},
		{"service", "redis", true},
		{"project", "cache", true},
		{"runtime", "docker", true},
		{"address", "127.0.0", true},
		{"status", "listening", true},
		{"origin", "user", true},
		{"protocol raw", "TCP", true},
		{"container name", "cache-1", true},
		{"container image", "7-alpine", true},
		{"compose project", "shop", true},
		{"compose service", "database", true},
		{"surrounding whitespace", "  redis  ", true},
		{"no match", "postgres", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entry.Matches(c.q); got != c.want {
				t.Errorf("Matches(%q) = %v, want %v", c.q, got, c.want)
			}
		})
	}
}
