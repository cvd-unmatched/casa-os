package service

import "testing"

func TestStripComposeGoTempDir(t *testing.T) {
	cases := []struct{ in, want string }{
		// Uncorrupted (a genuinely relative path, or already-clean input)
		// should pass through unchanged.
		{".", "."},
		{"./frontend", "./frontend"},
		// Corrupted by NewComposeAppFromYAML's throwaway temp dir - the
		// exact shape confirmed live: "unable to prepare context: path
		// \"/tmp/casaos-compose-app-250990937/frontend\" not found".
		{"/tmp/casaos-compose-app-250990937/frontend", "frontend"},
		{"/tmp/casaos-compose-app-250990937", "."},
		{`C:\Users\x\AppData\Local\Temp\casaos-compose-app-123456\frontend`, "frontend"},
		{"/tmp/casaos-compose-app-abc/nested/dir", "nested/dir"},
	}
	for _, c := range cases {
		if got := stripComposeGoTempDir(c.in); got != c.want {
			t.Errorf("stripComposeGoTempDir(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
