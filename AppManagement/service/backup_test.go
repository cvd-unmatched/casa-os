package service

import "testing"

// These cover the OS-independent parts of backup.go only. The staging/
// extraction path (stageEntry, checkFreeSpace, setTarOwnership) uses
// syscall.Stat_t/Statfs and os.Chown, which are Linux-only and can only be
// meaningfully exercised on an actual Linux host - not covered here.

func TestDataArchivePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/DATA/AppData/plex/config", "appdata/DATA/AppData/plex/config"},
		{"/srv/data", "appdata/srv/data"},
	}
	for _, c := range cases {
		if got := dataArchivePath(c.in); got != c.want {
			t.Errorf("dataArchivePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestComposeArchivePath(t *testing.T) {
	if got, want := composeArchivePath("plex"), "compose/plex/docker-compose.yml"; got != want {
		t.Errorf("composeArchivePath(%q) = %q, want %q", "plex", got, want)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{500, "500 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.n); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
