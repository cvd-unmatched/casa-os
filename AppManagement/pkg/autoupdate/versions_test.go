package autoupdate

import "testing"

func TestNewestTag(t *testing.T) {
	cases := []struct {
		name              string
		tags              []string
		includePrerelease bool
		want              string
		wantOK            bool
	}{
		{
			name:              "rc9 does not outrank rc18",
			tags:              []string{"v0.0.1-rc9", "v0.0.1-rc18", "v0.0.1-rc10"},
			includePrerelease: true,
			want:              "v0.0.1-rc18",
			wantOK:            true,
		},
		{
			name:              "plain semver picks the highest patch",
			tags:              []string{"1.2.3", "1.2.10", "1.2.9"},
			includePrerelease: false,
			want:              "1.2.10",
			wantOK:            true,
		},
		{
			name:              "a real release outranks a prerelease of the same core",
			tags:              []string{"v0.0.1-rc18", "v0.0.1"},
			includePrerelease: true,
			want:              "v0.0.1",
			wantOK:            true,
		},
		{
			name:              "prerelease excluded when not opted in",
			tags:              []string{"v0.0.1-rc18"},
			includePrerelease: false,
			want:              "",
			wantOK:            false,
		},
		{
			name:              "no parseable tags",
			tags:              []string{"latest", "main"},
			includePrerelease: true,
			want:              "",
			wantOK:            false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := NewestTag(c.tags, c.includePrerelease)
			if ok != c.wantOK || got != c.want {
				t.Errorf("NewestTag(%v, %v) = (%q, %v), want (%q, %v)", c.tags, c.includePrerelease, got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"rc9", "rc18", true},
		{"rc18", "rc9", false},
		{"rc9", "rc9", false},
		{"rc09", "rc9", false}, // leading zeros don't change numeric value
		{"rc2", "rc10", true},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
