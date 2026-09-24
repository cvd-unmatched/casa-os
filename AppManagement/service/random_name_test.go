package service

import "testing"

func TestRandomProjectNameIsAlwaysValid(t *testing.T) {
	for i := 0; i < 2000; i++ {
		if name := randomProjectName(); !composeProjectNameRE.MatchString(name) {
			t.Fatalf("generated an app name compose would reject: %q", name)
		}
	}
}
