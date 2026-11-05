package docker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func bearerChallenge(serverURL string) string {
	return `bearer realm="` + serverURL + `/token",service="registry.test"`
}

func TestGetBearerHeaderReportsARefusedLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"code":"DENIED","message":"denied"}]}`))
	}))
	defer server.Close()

	header, err := GetBearerHeader(bearerChallenge(server.URL), "registry.test/owner/image", "c3RhbGU=")
	if err == nil {
		t.Fatalf("a refused login must be an error, got header %q", header)
	}
	if !strings.Contains(err.Error(), "403 Forbidden") {
		t.Errorf("error should carry the registry's status, got %q", err)
	}
}

func TestGetBearerHeaderReturnsTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"abc123"}`))
	}))
	defer server.Close()

	header, err := GetBearerHeader(bearerChallenge(server.URL), "registry.test/owner/image", "")
	if err != nil {
		t.Fatal(err)
	}
	if header != "Bearer abc123" {
		t.Errorf("got %q", header)
	}
}
