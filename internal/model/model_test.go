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
