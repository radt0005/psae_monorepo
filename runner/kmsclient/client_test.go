package kmsclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveSuccess(t *testing.T) {
	var gotBody resolveRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resolve" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"prod-dsn": "postgres://x"})
	}))
	defer srv.Close()

	c := New(srv.URL)
	vals, err := c.Resolve(context.Background(), "tok", []string{"prod-dsn"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if vals["prod-dsn"] != "postgres://x" {
		t.Fatalf("values = %v", vals)
	}
	if gotBody.Token != "tok" || len(gotBody.Names) != 1 || gotBody.Names[0] != "prod-dsn" {
		t.Fatalf("request body = %+v", gotBody)
	}
}

func TestResolveErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := New(srv.URL).Resolve(context.Background(), "tok", []string{"db"}); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}
