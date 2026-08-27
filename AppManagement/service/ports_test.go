package service

import "testing"

func TestFirstPortNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"8080", 8080},
		{"80", 80},
		{"8080-8090", 8080},
		{"53/udp", 53},
		{"", 0},
		{"not-a-port", 0},
	}
	for _, c := range cases {
		if got := firstPortNumber(c.in); got != c.want {
			t.Errorf("firstPortNumber(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
