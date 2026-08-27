//go:build darwin

package platform

import "testing"

func TestAppleScriptString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", `""`},
		{"port 3000 is now up", `"port 3000 is now up"`},
		{`say "hi"`, `"say \"hi\""`},
		{`a\b`, `"a\\b"`},
	}
	for _, c := range cases {
		if got := appleScriptString(c.in); got != c.want {
			t.Errorf("appleScriptString(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
