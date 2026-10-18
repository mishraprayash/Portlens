package version

import "testing"

func TestFromBuildInfo(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v0.3.1", "0.3.1"},
		{"v0.3.1-0.20250101-abcdef", "0.3.1-0.20250101-abcdef"},
		{"(devel)", "dev"},
		{"", "dev"},
	}
	for _, c := range cases {
		if got := fromBuildInfo(c.in, "dev"); got != c.want {
			t.Errorf("fromBuildInfo(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
