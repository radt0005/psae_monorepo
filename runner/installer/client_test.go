package installer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "secret-token", nil)
	if _, err := c.get(context.Background(), "/anything"); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want Bearer secret-token", gotAuth)
	}
}

func TestClientNon2xxIsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "t", nil)
	_, err := c.get(context.Background(), "/missing")
	var he *httpError
	if !asHTTPError(err, &he) || he.Status != http.StatusNotFound {
		t.Fatalf("expected 404 httpError, got %v", err)
	}
}

// asHTTPError is a tiny errors.As wrapper kept local to the test.
func asHTTPError(err error, target **httpError) bool {
	he, ok := err.(*httpError)
	if ok {
		*target = he
	}
	return ok
}
