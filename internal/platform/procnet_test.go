package platform

import "testing"

func TestDecodeAddr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0100007F", "127.0.0.1"},
		{"00000000", "0.0.0.0"},
		{"00000000000000000000000001000000", "::1"},
		{"00000000000000000000000000000000", "::"},
	}
	for _, c := range cases {
		if got := decodeAddr(c.in); got != c.want {
			t.Errorf("decodeAddr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseProcNetContent(t *testing.T) {
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000   501        0 12345 1 0000000000000000 100 0 0 10 0
   1: 0100007F:0BB8 0100007F:C000 01 00000000:00000000 00:00000000 00000000   501        0 54321 1 0000000000000000 100 0 0 10 0
`
	rows := parseProcNetContent(content)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	r0 := rows[0]
	if r0.localAddr != "127.0.0.1" || r0.localPort != 3000 {
		t.Errorf("r0 = %+v, want local 127.0.0.1:3000", r0)
	}
	if r0.state != "0A" {
		t.Errorf("r0.state = %q, want 0A (LISTEN)", r0.state)
	}
	if r0.inode != 12345 {
		t.Errorf("r0.inode = %d, want 12345", r0.inode)
	}
	if r0.uid != 501 {
		t.Errorf("r0.uid = %d, want 501", r0.uid)
	}

	r1 := rows[1]
	if r1.remoteAddr != "127.0.0.1" || r1.remotePort != 49152 {
		t.Errorf("r1 = %+v, want remote 127.0.0.1:49152", r1)
	}
	if tcpStateNames[r1.state] != "ESTABLISHED" {
		t.Errorf("r1 state = %q, want ESTABLISHED", tcpStateNames[r1.state])
	}
}

func TestParseProcNetContentIgnoresMalformed(t *testing.T) {
	content := "  sl  local_address rem_address st\n   0: ZZZZ:0BB8 00000000:0000 0A junk\n"
	if rows := parseProcNetContent(content); len(rows) != 0 {
		t.Fatalf("got %d rows, want 0", len(rows))
	}
}
