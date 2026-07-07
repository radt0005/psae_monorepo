package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"captoken"

	"spade_kms/internal/envelope"
	"spade_kms/internal/store"
	"spade_kms/internal/token"
)

type fakeVerifier struct {
	claims token.Claims
	err    error
}

func (f fakeVerifier) Verify(string) (token.Claims, error) { return f.claims, f.err }

func newTestServer(t *testing.T, v token.Verifier) *Server {
	t.Helper()
	st, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	k := make([]byte, 32)
	rand.Read(k)
	ks, err := envelope.NewKeySet(map[string][]byte{"v1": k}, "v1")
	if err != nil {
		t.Fatal(err)
	}
	return New(st, ks, v, nil)
}

func do(t *testing.T, s *Server, method, path, user string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-Spade-User-Id", user)
	}
	rec := httptest.NewRecorder()
	s.Echo().ServeHTTP(rec, req)
	return rec
}

func validVerifier(names ...string) fakeVerifier {
	return fakeVerifier{claims: token.Claims{
		UserID: "alice", InvocationID: "inv1",
		SecretNames: names, Expiry: time.Now().Add(time.Minute),
	}}
}

func TestSetListResolveDelete(t *testing.T) {
	s := newTestServer(t, validVerifier("db"))

	if rec := do(t, s, http.MethodPut, "/secrets/db", "alice", map[string]string{"value": "postgres://x"}); rec.Code != http.StatusNoContent {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}

	rec := do(t, s, http.MethodGet, "/secrets", "alice", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var listResp struct {
		Names []string `json:"names"`
	}
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	if len(listResp.Names) != 1 || listResp.Names[0] != "db" {
		t.Fatalf("list = %v", listResp.Names)
	}

	// Worker resolve path: capability token exchanged for values.
	rec = do(t, s, http.MethodPost, "/resolve", "", map[string]any{"token": "t", "names": []string{"db"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve: %d %s", rec.Code, rec.Body)
	}
	var vals map[string]string
	json.Unmarshal(rec.Body.Bytes(), &vals)
	if vals["db"] != "postgres://x" {
		t.Fatalf("resolve value = %v", vals)
	}

	if rec := do(t, s, http.MethodDelete, "/secrets/db", "alice", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestManagementRequiresAuth(t *testing.T) {
	s := newTestServer(t, validVerifier("db"))
	if rec := do(t, s, http.MethodPut, "/secrets/db", "", map[string]string{"value": "x"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if rec := do(t, s, http.MethodGet, "/secrets", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestResolveRejectsOutOfScope(t *testing.T) {
	// Token scopes only "db"; the request asks for "other" (which exists).
	s := newTestServer(t, validVerifier("db"))
	do(t, s, http.MethodPut, "/secrets/other", "alice", map[string]string{"value": "y"})
	rec := do(t, s, http.MethodPost, "/resolve", "", map[string]any{"token": "t", "names": []string{"other"}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 for out-of-scope name, got %d %s", rec.Code, rec.Body)
	}
}

func TestResolveRejectsBadToken(t *testing.T) {
	s := newTestServer(t, fakeVerifier{err: token.ErrUnconfigured})
	rec := do(t, s, http.MethodPost, "/resolve", "", map[string]any{"token": "t", "names": []string{"db"}})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for bad token, got %d", rec.Code)
	}
}

// TestResolveWithRealEd25519Token exercises the full cloud resolve path with a
// genuine scheduler-signed token: captoken sign → Ed25519Verifier → envelope
// decrypt → value, plus cross-user isolation (a token for another user cannot
// read alice's secret).
func TestResolveWithRealEd25519Token(t *testing.T) {
	pub, priv, _ := captoken.GenerateKey()
	s := newTestServer(t, token.NewEd25519Verifier([]ed25519.PublicKey{pub}))
	signer := captoken.NewSigner(priv)

	// alice stores a secret.
	if rec := do(t, s, http.MethodPut, "/secrets/prod-dsn", "alice", map[string]string{"value": "postgres://real"}); rec.Code != http.StatusNoContent {
		t.Fatalf("set: %d", rec.Code)
	}

	// The scheduler mints a token scoped to prod-dsn for alice.
	tok, _ := signer.Sign(captoken.Claims{
		UserID: "alice", InvocationID: "inv1",
		SecretNames: []string{"prod-dsn"}, Expiry: time.Now().Add(time.Minute),
	})
	rec := do(t, s, http.MethodPost, "/resolve", "", map[string]any{"token": tok, "names": []string{"prod-dsn"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve: %d %s", rec.Code, rec.Body)
	}
	var vals map[string]string
	json.Unmarshal(rec.Body.Bytes(), &vals)
	if vals["prod-dsn"] != "postgres://real" {
		t.Fatalf("resolved value = %v", vals)
	}

	// A validly-signed token for a different user resolves against THAT user's
	// namespace, so it cannot read alice's secret (404, not a leak).
	bobTok, _ := signer.Sign(captoken.Claims{
		UserID: "bob", InvocationID: "inv2",
		SecretNames: []string{"prod-dsn"}, Expiry: time.Now().Add(time.Minute),
	})
	rec = do(t, s, http.MethodPost, "/resolve", "", map[string]any{"token": bobTok, "names": []string{"prod-dsn"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob resolving alice's secret should 404, got %d %s", rec.Code, rec.Body)
	}
}

func TestResolveRejectsExpired(t *testing.T) {
	v := fakeVerifier{claims: token.Claims{
		UserID: "alice", SecretNames: []string{"db"}, Expiry: time.Now().Add(-time.Minute),
	}}
	s := newTestServer(t, v)
	do(t, s, http.MethodPut, "/secrets/db", "alice", map[string]string{"value": "x"})
	rec := do(t, s, http.MethodPost, "/resolve", "", map[string]any{"token": "t", "names": []string{"db"}})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for expired token, got %d", rec.Code)
	}
}
