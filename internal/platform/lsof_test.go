package platform

import "testing"

func TestParseLsofFields(t *testing.T) {
	input := "p938\ncredis-server\nf6\ntIPv4\nn127.0.0.1:6379\nf7\ntIPv6\nn[::1]:6379\n"
	records := parseLsofFields(input)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	r0 := records[0]
	if r0.pid != 938 {
		t.Errorf("r0.pid = %d, want 938", r0.pid)
	}
	if r0.command != "redis-server" {
		t.Errorf("r0.command = %q, want redis-server", r0.command)
	}
	if r0.family != "IPv4" || r0.name != "127.0.0.1:6379" {
		t.Errorf("r0 = %+v", r0)
	}

	r1 := records[1]
	if r1.pid != 938 {
		t.Errorf("r1.pid = %d, want 938 (pid must carry across sockets)", r1.pid)
	}
	if r1.family != "IPv6" || r1.name != "[::1]:6379" {
		t.Errorf("r1 = %+v", r1)
	}
}

func TestParseLsofFieldsConnection(t *testing.T) {
	input := "p661\ncrapportd\nf18\ntIPv6\nn[fe80:b::30:4dc7]:61670->[fe80:b::5a:dc22]:55115\nTST=ESTABLISHED\n"
	records := parseLsofFields(input)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].state != "ESTABLISHED" {
		t.Errorf("state = %q, want ESTABLISHED", records[0].state)
	}
}

func TestParseSockName(t *testing.T) {
	cases := []struct {
		in   string
		addr string
		port uint16
		ok   bool
	}{
		{"127.0.0.1:6379", "127.0.0.1", 6379, true},
		{"*:61670", "*", 61670, true},
		{"[::1]:6379", "::1", 6379, true},
		{"0.0.0.0:3000", "0.0.0.0", 3000, true},
		{"", "", 0, false},
		{"notasock", "", 0, false},
		{"127.0.0.1:notaport", "", 0, false},
	}
	for _, c := range cases {
		addr, port, ok := parseSockName(c.in)
		if ok != c.ok || addr != c.addr || port != c.port {
			t.Errorf("parseSockName(%q) = (%q, %d, %v), want (%q, %d, %v)",
				c.in, addr, port, ok, c.addr, c.port, c.ok)
		}
	}
}

func TestNormalizeAddr(t *testing.T) {
	cases := []struct{ in, family, want string }{
		{"*", "IPv4", "0.0.0.0"},
		{"*", "IPv6", "::"},
		{"127.0.0.1", "IPv4", "127.0.0.1"},
		{"::1", "IPv6", "::1"},
	}
	for _, c := range cases {
		if got := normalizeAddr(c.in, c.family); got != c.want {
			t.Errorf("normalizeAddr(%q, %q) = %q, want %q", c.in, c.family, got, c.want)
		}
	}
}
