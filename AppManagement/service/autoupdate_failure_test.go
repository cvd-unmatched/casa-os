package service

import (
	"errors"
	"strings"
	"testing"
)

func TestDescribeTagCheckError(t *testing.T) {
	cases := []struct {
		name  string
		image string
		err   error
		want  string
	}{
		{"revoked token", "ghcr.io/cvd-unmatched/movies:0.1.0", errors.New(`registry responded to tags list with "403 Forbidden"`), "docker login ghcr.io"},
		{"unauthorized", "ghcr.io/cvd-unmatched/movies:0.1.0", errors.New(`registry responded to tags list with "401 Unauthorized"`), "rejected the login"},
		{"no credentials", "ghcr.io/cvd-unmatched/movies:0.1.0", errors.New("no credentials available"), "docker login ghcr.io"},
		{"network", "ghcr.io/cvd-unmatched/movies:0.1.0", errors.New("dial tcp: i/o timeout"), "could not list tags from ghcr.io"},
	}
	for _, c := range cases {
		got := describeTagCheckError(c.image, c.err)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: %q does not contain %q", c.name, got, c.want)
		}
	}
	if got := describeTagCheckError("ghcr.io/x/y:1", errors.New("dial tcp: i/o timeout")); strings.Contains(got, "docker login") {
		t.Errorf("a network error must not tell the user to log in again: %q", got)
	}
}
