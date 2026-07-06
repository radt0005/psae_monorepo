package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"core"
)

type staticKeys []string

func (s staticKeys) Keys(context.Context) ([]string, error) { return []string(s), nil }

// fakeRegistry serves a single signed artifact and counts tarball downloads.
type fakeRegistry struct {
	tarball  []byte
	sig      []byte
	pubB64   string
	hash     string
	state    string
	down     atomic.Bool
	tarballs atomic.Int32
}

func newFakeRegistry(t *testing.T, state string) *fakeRegistry {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	// A minimal Go collection: go.mod marker + one block manifest.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"go.mod":            "module hello\n",
		"blocks/greet.yaml": "id: hello.greet\nentrypoint: greet\nkind: standard\n",
	}
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := buf.Bytes()
	sum := sha256.Sum256(tarball)
	return &fakeRegistry{
		tarball: tarball,
		sig:     ed25519.Sign(priv, tarball),
		pubB64:  base64.StdEncoding.EncodeToString(pub),
		hash:    hex.EncodeToString(sum[:]),
		state:   state,
	}
}

func (f *fakeRegistry) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/pubkeys", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string][]string{"keys": {f.pubB64}})
	})
	mux.HandleFunc("/artifacts/", func(w http.ResponseWriter, r *http.Request) {
		if f.down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		switch {
		case hasSuffix(r.URL.Path, "/meta"):
			json.NewEncoder(w).Encode(map[string]string{
				"version": "1.0.0", "platform": "linux", "arch": "amd64",
				"content_hash": f.hash, "state": f.state,
			})
		case hasSuffix(r.URL.Path, ".tar.gz"):
			f.tarballs.Add(1)
			w.Write(f.tarball)
		case hasSuffix(r.URL.Path, ".sig"):
			w.Write(f.sig)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func hasSuffix(s, suf string) bool { return len(s) >= len(suf) && s[len(s)-len(suf):] == suf }

func newTestInstaller(t *testing.T, f *fakeRegistry, srvURL string) *Installer {
	t.Helper()
	reg, err := core.OpenRegistry(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	client := NewClient(srvURL, "worker-token", nil)
	return New(client, staticKeys{f.pubB64}, reg, t.TempDir())
}

func TestInstallHappyPath(t *testing.T) {
	f := newFakeRegistry(t, "available")
	in := newTestInstaller(t, f, f.server(t).URL)

	if err := in.Install(context.Background(), "hello", "1.0.0"); err != nil {
		t.Fatalf("install: %v", err)
	}

	// The block is indexed with registry provenance.
	entry, err := in.Registry.LookupBlock("hello.greet", "1.0.0")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if entry.Source != core.InstallSourceRegistry {
		t.Errorf("Source = %q, want registry", entry.Source)
	}
	if entry.Signature == "" || entry.RegistryState != "available" {
		t.Errorf("provenance not recorded: %+v", entry)
	}
	// The artifact was unpacked into <blocksDir>/hello/1.0.0.
	if got := filepath.Base(filepath.Dir(entry.InstalledPath)); got != "hello" {
		t.Errorf("InstalledPath = %q", entry.InstalledPath)
	}
}

func TestInstallRejectsBadSignature(t *testing.T) {
	f := newFakeRegistry(t, "available")
	f.sig = ed25519.Sign(ed25519.NewKeyFromSeed(make([]byte, 32)), []byte("wrong")) // signature from an untrusted key
	in := newTestInstaller(t, f, f.server(t).URL)

	err := in.Install(context.Background(), "hello", "1.0.0")
	if !IsRejected(err) {
		t.Fatalf("expected Rejected, got %v", err)
	}
	assertNothingIndexed(t, in)
}

func TestInstallRejectsHashMismatch(t *testing.T) {
	f := newFakeRegistry(t, "available")
	f.hash = "deadbeef" // meta advertises a hash that does not match the bytes
	in := newTestInstaller(t, f, f.server(t).URL)

	err := in.Install(context.Background(), "hello", "1.0.0")
	if !IsRejected(err) {
		t.Fatalf("expected Rejected, got %v", err)
	}
	assertNothingIndexed(t, in)
}

func TestInstallRejectsRecalledState(t *testing.T) {
	f := newFakeRegistry(t, "recalled")
	in := newTestInstaller(t, f, f.server(t).URL)

	err := in.Install(context.Background(), "hello", "1.0.0")
	if !IsRejected(err) {
		t.Fatalf("expected Rejected for recalled state, got %v", err)
	}
	if f.tarballs.Load() != 0 {
		t.Errorf("must not download a recalled artifact; got %d fetches", f.tarballs.Load())
	}
	assertNothingIndexed(t, in)
}

func TestInstallTransientErrorNotRejected(t *testing.T) {
	f := newFakeRegistry(t, "available")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	in := newTestInstaller(t, f, srv.URL)

	err := in.Install(context.Background(), "hello", "1.0.0")
	if err == nil || IsRejected(err) {
		t.Fatalf("5xx must be a transient (non-Rejected) error, got %v", err)
	}
}

func TestConcurrentInstallFetchesOnce(t *testing.T) {
	f := newFakeRegistry(t, "available")
	in := newTestInstaller(t, f, f.server(t).URL)

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = in.Install(context.Background(), "hello", "1.0.0")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	if n := f.tarballs.Load(); n != 1 {
		t.Errorf("expected exactly 1 tarball fetch, got %d", n)
	}
}

func TestRecheckAvailableBumpsFreshness(t *testing.T) {
	f := newFakeRegistry(t, "available")
	in := newTestInstaller(t, f, f.server(t).URL)
	if err := in.Install(context.Background(), "hello", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	before, _ := in.Registry.LookupBlock("hello.greet", "1.0.0")

	if err := in.Recheck(context.Background(), "hello", "1.0.0"); err != nil {
		t.Fatalf("recheck of an available version should succeed: %v", err)
	}
	after, _ := in.Registry.LookupBlock("hello.greet", "1.0.0")
	if !after.LastVerifiedAt.After(before.LastVerifiedAt) {
		t.Errorf("recheck should bump LastVerifiedAt (%v -> %v)", before.LastVerifiedAt, after.LastVerifiedAt)
	}
}

func TestRecheckRecalledEvicts(t *testing.T) {
	f := newFakeRegistry(t, "available")
	in := newTestInstaller(t, f, f.server(t).URL)
	if err := in.Install(context.Background(), "hello", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	// The registry recalls the version after install.
	f.state = "recalled"

	err := in.Recheck(context.Background(), "hello", "1.0.0")
	if !IsRejected(err) {
		t.Fatalf("recalled recheck should be Rejected, got %v", err)
	}
	// The index entry and on-disk install are evicted.
	if _, err := in.Registry.LookupBlock("hello.greet", "1.0.0"); err == nil {
		t.Error("recalled block should be removed from the index")
	}
	if _, err := os.Stat(filepath.Join(in.BlocksDir, "hello", "1.0.0")); !os.IsNotExist(err) {
		t.Error("recalled install directory should be removed")
	}
}

func TestRecheckTransientKeepsInstall(t *testing.T) {
	f := newFakeRegistry(t, "available")
	in := newTestInstaller(t, f, f.server(t).URL)
	if err := in.Install(context.Background(), "hello", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	f.down.Store(true) // registry unreachable

	err := in.Recheck(context.Background(), "hello", "1.0.0")
	if err == nil || IsRejected(err) {
		t.Fatalf("a 5xx recheck must be transient (non-Rejected), got %v", err)
	}
	// The install is left in place for best-effort execution.
	if _, err := in.Registry.LookupBlock("hello.greet", "1.0.0"); err != nil {
		t.Error("transient recheck must not evict the install")
	}
}

func assertNothingIndexed(t *testing.T, in *Installer) {
	t.Helper()
	blocks, err := in.Registry.ListBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 0 {
		t.Errorf("expected empty index after rejection, got %d entries", len(blocks))
	}
}
