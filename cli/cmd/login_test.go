package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// --- requestDeviceCode ---

func TestRequestDeviceCode_Success(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/auth/device/code" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		if body["client_id"] != clientID {
			t.Errorf("expected client_id %q, got %q", clientID, body["client_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deviceCodeResponse{
			DeviceCode:      "dev-code-abc",
			UserCode:        "ABCD-EFGH",
			VerificationURI: srv.URL + "/device",
			ExpiresIn:       1800,
			Interval:        5,
		})
	}))
	defer srv.Close()

	dc, err := requestDeviceCode(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dc.DeviceCode != "dev-code-abc" {
		t.Errorf("DeviceCode = %q, want %q", dc.DeviceCode, "dev-code-abc")
	}
	if dc.UserCode != "ABCD-EFGH" {
		t.Errorf("UserCode = %q, want %q", dc.UserCode, "ABCD-EFGH")
	}
	if dc.Interval != 5 {
		t.Errorf("Interval = %d, want 5", dc.Interval)
	}
}

func TestRequestDeviceCode_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := requestDeviceCode(srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- doTokenRequest ---

func TestDoTokenRequest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "urn:ietf:params:oauth:grant-type:device_code" {
			t.Errorf("unexpected grant_type: %q", body["grant_type"])
		}
		if body["client_id"] != clientID {
			t.Errorf("unexpected client_id: %q", body["client_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok-xyz", TokenType: "bearer"})
	}))
	defer srv.Close()

	token, errCode, err := doTokenRequest(srv.Client(), srv.URL, "dev-code-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tok-xyz" {
		t.Errorf("token = %q, want %q", token, "tok-xyz")
	}
	if errCode != "" {
		t.Errorf("errCode = %q, want empty", errCode)
	}
}

func TestDoTokenRequest_Pending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "authorization_pending"})
	}))
	defer srv.Close()

	token, errCode, err := doTokenRequest(srv.Client(), srv.URL, "dev-code-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if errCode != "authorization_pending" {
		t.Errorf("errCode = %q, want %q", errCode, "authorization_pending")
	}
}

func TestDoTokenRequest_SlowDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "slow_down"})
	}))
	defer srv.Close()

	_, errCode, err := doTokenRequest(srv.Client(), srv.URL, "dev-code-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errCode != "slow_down" {
		t.Errorf("errCode = %q, want %q", errCode, "slow_down")
	}
}

func TestDoTokenRequest_ExpiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "expired_token"})
	}))
	defer srv.Close()

	_, errCode, err := doTokenRequest(srv.Client(), srv.URL, "dev-code-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errCode != "expired_token" {
		t.Errorf("errCode = %q, want %q", errCode, "expired_token")
	}
}

func TestDoTokenRequest_AccessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "access_denied"})
	}))
	defer srv.Close()

	_, errCode, err := doTokenRequest(srv.Client(), srv.URL, "dev-code-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errCode != "access_denied" {
		t.Errorf("errCode = %q, want %q", errCode, "access_denied")
	}
}

// --- Credentials round-trip ---

func TestSaveLoadCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	want := Credentials{
		Token:     "my-token",
		ServerURL: "https://example.com",
		IssuedAt:  time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	}
	if err := SaveCredentials(want); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got.Token != want.Token {
		t.Errorf("Token = %q, want %q", got.Token, want.Token)
	}
	if got.ServerURL != want.ServerURL {
		t.Errorf("ServerURL = %q, want %q", got.ServerURL, want.ServerURL)
	}
	if !got.IssuedAt.Equal(want.IssuedAt) {
		t.Errorf("IssuedAt = %v, want %v", got.IssuedAt, want.IssuedAt)
	}
}

func TestLoadCredentials_NotLoggedIn(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	_, err := LoadCredentials()
	if err == nil {
		t.Fatal("expected error when no credentials file exists")
	}
}

func TestSaveCredentials_FileMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	if err := SaveCredentials(Credentials{Token: "tok", ServerURL: "https://x.com", IssuedAt: time.Now()}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	info, err := os.Stat(CredentialsPath())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}
}
