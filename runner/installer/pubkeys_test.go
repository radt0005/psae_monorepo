package installer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"core"
)

// keyServer serves /pubkeys with a swappable key list and an on/off switch.
type keyServer struct {
	keys atomic.Value // []string
	down atomic.Bool
	hits atomic.Int32
}

func (k *keyServer) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/pubkeys", func(w http.ResponseWriter, r *http.Request) {
		k.hits.Add(1)
		if k.down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		keys, _ := k.keys.Load().([]string)
		json.NewEncoder(w).Encode(map[string][]string{"keys": keys})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestPubKeyCacheFetchesAndPersists(t *testing.T) {
	ks := &keyServer{}
	ks.keys.Store([]string{"key-a", "key-b"})
	srv := ks.server(t)
	cachePath := filepath.Join(t.TempDir(), "pubkeys.json")
	cache := NewPubKeyCache(NewClient(srv.URL, "tok", nil), cachePath)

	got, err := cache.Keys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 keys, got %v", got)
	}
	// It persisted to disk for use across restarts.
	data, err := readJSONKeys(cachePath)
	if err != nil || len(data) != 2 {
		t.Fatalf("cache not persisted: %v / %v", data, err)
	}
	// A warm cache serves from memory without hitting the network again.
	before := ks.hits.Load()
	_, _ = cache.Keys(context.Background())
	if ks.hits.Load() != before {
		t.Error("warm cache should not re-fetch")
	}
}

func TestPubKeyCacheFallsBackThroughOutage(t *testing.T) {
	ks := &keyServer{}
	ks.keys.Store([]string{"key-a"})
	srv := ks.server(t)
	cache := NewPubKeyCache(NewClient(srv.URL, "tok", nil), "")

	if _, err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Registry goes down: a refresh keeps the last good keys rather than erroring.
	ks.down.Store(true)
	got, err := cache.Refresh(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("expected cached keys during outage, got %v / %v", got, err)
	}
}

func TestPubKeyCacheLoadsFromDiskWhenColdAndServerDown(t *testing.T) {
	// Pre-seed a disk cache, then start cold against a dead registry.
	cachePath := filepath.Join(t.TempDir(), "pubkeys.json")
	writeJSONKeys(t, cachePath, []string{"disk-key"})
	ks := &keyServer{}
	ks.down.Store(true)
	srv := ks.server(t)
	cache := NewPubKeyCache(NewClient(srv.URL, "tok", nil), cachePath)

	got, err := cache.Keys(context.Background())
	if err != nil || len(got) != 1 || got[0] != "disk-key" {
		t.Fatalf("expected disk fallback, got %v / %v", got, err)
	}
}

func TestPubKeyCacheServesRotationSet(t *testing.T) {
	// During rotation the endpoint returns old+new; an artifact signed by the new
	// key verifies via the cached list (core.VerifySignature accepts any).
	oldPub, _, _ := ed25519.GenerateKey(rand.Reader)
	newPub, newPriv, _ := ed25519.GenerateKey(rand.Reader)
	b64 := func(k []byte) string { return base64.StdEncoding.EncodeToString(k) }

	ks := &keyServer{}
	ks.keys.Store([]string{b64(oldPub), b64(newPub)})
	srv := ks.server(t)
	cache := NewPubKeyCache(NewClient(srv.URL, "tok", nil), "")

	keys, err := cache.Keys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("artifact")
	if !core.VerifySignature(keys, data, ed25519.Sign(newPriv, data)) {
		t.Error("new-key signature should verify against the rotation set")
	}
}

func readJSONKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keys []string
	err = json.Unmarshal(data, &keys)
	return keys, err
}

func writeJSONKeys(t *testing.T, path string, keys []string) {
	t.Helper()
	data, _ := json.Marshal(keys)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
