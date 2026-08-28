package platform

import (
	"os"
	"testing"
)

// sampleProcNet is representative /proc/net/tcp content: a loopback listener
// (127.0.0.1:3000) and a wildcard listener (*:8080), both in LISTEN (0A).
const sampleProcNet = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:0B7C 00000000:0000 0A 00000000:00000000 000:00000000 00000000  501        0 1234567 1 0000000000000000 100 0 0 10 0
   1: 00000000:1F90 00000000:0000 0A 00000000:00000000 000:00000000 00000000  501        0 7654321 1 0000000000000000 100 0 0 10 0
   2: 0100007F:E2C3 0100007F:0B7C 01 00000000:00000000 000:00000000 00000000  501        0 9999999 1 0000000000000000 20 4 30 10 -1
`

func BenchmarkParseProcNet(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rows := parseProcNetContent(sampleProcNet)
		if len(rows) != 3 {
			b.Fatalf("parsed %d rows, want 3", len(rows))
		}
	}
}

func BenchmarkReadProcNetFile(b *testing.B) {
	dir := b.TempDir()
	path := dir + "/tcp"
	if err := os.WriteFile(path, []byte(sampleProcNet), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := parseProcNet(path); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeAddr(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := decodeAddr("0100007F"); got != "127.0.0.1" {
			b.Fatalf("decode = %q", got)
		}
		if got := decodeAddr("00000000000000000000000001000000"); got != "::1" {
			b.Fatalf("decode6 = %q", got)
		}
	}
}
