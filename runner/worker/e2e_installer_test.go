package worker

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
	"strings"
	"testing"
	"time"

	"core"
	spade "spade_runner"
	"spade_runner/installer"

	"github.com/google/uuid"
)

// These tests wire the *real* installer.Installer behind the worker against a
// fake signed registry, exercising the full miss → fetch → verify → unpack →
// index → run seam (with a fake executor, so no isolate is needed).

type e2eArtifact struct {
	tarball []byte
	sig     []byte
	pubB64  string
	hash    string
}

func buildSignedArtifact(t *testing.T) e2eArtifact {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"go.mod":            "module e2e\n",
		"blocks/greet.yaml": "id: e2e.greet\nentrypoint: greet\nkind: standard\ninputs: {}\noutputs: {}\n",
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
	return e2eArtifact{
		tarball: tarball,
		sig:     ed25519.Sign(priv, tarball),
		pubB64:  base64.StdEncoding.EncodeToString(pub),
		hash:    hex.EncodeToString(sum[:]),
	}
}

// e2eServer serves the signed artifact with a mutable version state.
func e2eServer(t *testing.T, art e2eArtifact, state *string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/pubkeys", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string][]string{"keys": {art.pubB64}})
	})
	mux.HandleFunc("/artifacts/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/meta"):
			json.NewEncoder(w).Encode(map[string]string{
				"version": "1.0.0", "platform": "linux", "arch": "amd64",
				"content_hash": art.hash, "state": *state,
			})
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			w.Write(art.tarball)
		case strings.HasSuffix(r.URL.Path, ".sig"):
			w.Write(art.sig)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type e2eKeys struct{ k []string }

func (s e2eKeys) Keys(context.Context) ([]string, error) { return s.k, nil }

func e2eJob(root string) spade.Job {
	blockID := uuid.New()
	pipeID := uuid.New()
	return spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID:      blockID.String(),
			BlockName:         "e2e.greet",
			CollectionVersion: "1.0.0",
			PipelineID:        pipeID,
			WorkDir:           filepath.Join(root, "work"),
		},
		Pipeline: core.Pipeline{
			Id:     pipeID,
			Blocks: []core.PipelineBlock{{Id: blockID, Name: "e2e.greet", Version: "1.0.0"}},
		},
		Manifests: map[string]core.BlockManifest{},
	}
}

func TestE2E_MissInstallRun(t *testing.T) {
	art := buildSignedArtifact(t)
	state := "available"
	srv := e2eServer(t, art, &state)

	reg, root := setupRegistry(t)
	blocksDir := filepath.Join(root, "blocks")
	client := installer.NewClient(srv.URL, "worker-token", nil)
	inst := installer.New(client, e2eKeys{[]string{art.pubB64}}, reg, blocksDir)

	fake := &fakeExecutor{result: core.BlockInvocationResult{Status: core.ExecutionStatusComplete}}
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake), WithInstaller(inst), WithFreshness(time.Hour))

	res, err := w.Run(context.Background(), e2eJob(root))
	if err != nil {
		t.Fatalf("unexpected infra error: %v", err)
	}
	if res.Status != core.ExecutionStatusComplete {
		t.Fatalf("expected Complete after real install, got %s: %s", res.Status, res.Error)
	}
	// The block was really fetched, verified, unpacked, and indexed with provenance.
	entry, err := reg.LookupBlock("e2e.greet", "1.0.0")
	if err != nil {
		t.Fatalf("block not indexed: %v", err)
	}
	if entry.Source != core.InstallSourceRegistry || entry.Signature == "" {
		t.Errorf("expected registry provenance, got %+v", entry)
	}
	if _, err := os.Stat(filepath.Join(blocksDir, "e2e", "1.0.0", "blocks", "greet.yaml")); err != nil {
		t.Errorf("artifact not unpacked to disk: %v", err)
	}
}

func TestE2E_RecallEvictsOnRecheck(t *testing.T) {
	art := buildSignedArtifact(t)
	state := "available"
	srv := e2eServer(t, art, &state)

	reg, root := setupRegistry(t)
	blocksDir := filepath.Join(root, "blocks")
	client := installer.NewClient(srv.URL, "worker-token", nil)
	inst := installer.New(client, e2eKeys{[]string{art.pubB64}}, reg, blocksDir)
	fake := &fakeExecutor{result: core.BlockInvocationResult{Status: core.ExecutionStatusComplete}}

	// First run installs and executes.
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake), WithInstaller(inst), WithFreshness(time.Hour))
	if res, err := w.Run(context.Background(), e2eJob(root)); err != nil || res.Status != core.ExecutionStatusComplete {
		t.Fatalf("first run should install+run: %v / %s", err, res.Status)
	}

	// The registry recalls the version; age the index entry so the next run
	// re-checks freshness.
	state = "recalled"
	if err := reg.TouchCollection("e2e", "1.0.0", "available", time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Second run: stale entry → recheck → recalled → refuse + evict.
	w2 := New(reg, filepath.Join(root, "work"), WithExecutor(fake), WithInstaller(inst), WithFreshness(time.Minute))
	res, err := w2.Run(context.Background(), e2eJob(root))
	if err != nil {
		t.Fatalf("recall must be a block failure, got infra error: %v", err)
	}
	if res.Status != core.ExecutionStatusError || !strings.Contains(res.Error, "recalled") {
		t.Fatalf("expected recalled block failure, got %s: %s", res.Status, res.Error)
	}
	if _, err := reg.LookupBlock("e2e.greet", "1.0.0"); err == nil {
		t.Error("recalled block should be evicted from the index")
	}
	if _, err := os.Stat(filepath.Join(blocksDir, "e2e", "1.0.0")); !os.IsNotExist(err) {
		t.Error("recalled install dir should be removed")
	}
}
