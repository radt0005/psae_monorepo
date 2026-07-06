package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"spade_registry/internal/audit"
	"spade_registry/internal/auth"
	"spade_registry/internal/blob"
	"spade_registry/internal/config"
	"spade_registry/internal/sign"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
	"spade_registry/internal/wire"
)

// fakeDev is a stub developer verifier keyed on a single token.
type fakeDev struct {
	token string
	user  string
}

func (f fakeDev) Verify(tok string) (auth.Developer, error) {
	if tok != "" && tok == f.token {
		return auth.Developer{UserID: f.user}, nil
	}
	return auth.Developer{}, auth.ErrUnauthenticated
}

type harness struct {
	srv    *Server
	echo   *echo.Echo
	store  *store.Store
	blob   blob.Store
	keyset *sign.Keyset
	cfg    config.RegistryConfig
}

func newHarness(t *testing.T, admins ...string) *harness {
	t.Helper()
	st, err := store.OpenSQLite(t.TempDir() + "/api.db")
	require.NoError(t, err)
	bs, err := blob.NewFSStore(t.TempDir() + "/blobs")
	require.NoError(t, err)
	ks := sign.NewKeyset(st)
	_, err = ks.EnsureActiveKey()
	require.NoError(t, err)

	cfg := config.RegistryConfig{
		StagingPrefix:  "staging/",
		ArtifactPrefix: "artifacts/",
		AdminUserIDs:   admins,
	}
	srv := New(Options{
		Config: cfg,
		Store:  st,
		Keyset: ks,
		Audit:  audit.New(st),
		State:  state.New(st, audit.New(st), nil),
		Blob:   bs,
		Dev:    fakeDev{token: "dev-token", user: "user-1"},
	})
	return &harness{srv: srv, echo: srv.Routes(), store: st, blob: bs, keyset: ks, cfg: cfg}
}

func (h *harness) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		data, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.echo.ServeHTTP(w, r)
	return w
}

func TestPublishRequiresDeveloper(t *testing.T) {
	h := newHarness(t)
	w := h.do(t, http.MethodPost, "/publish", "", wire.PublishRequest{
		RepoURL: "file:///x", CommitSHA: "abc", Collection: "c", Version: "1.0.0",
	})
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPublishCreatesVersionAndJob(t *testing.T) {
	h := newHarness(t)
	w := h.do(t, http.MethodPost, "/publish", "dev-token", wire.PublishRequest{
		RepoURL: "file:///x", CommitSHA: "abc", Collection: "gdal", Version: "1.0.0", Language: "go",
	})
	require.Equal(t, http.StatusAccepted, w.Code)

	var resp wire.PublishResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.VersionID)
	require.Equal(t, string(store.StateSubmitted), resp.State)

	v, err := h.store.GetVersion("gdal", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, store.StateSubmitted, v.State)

	// A queued build job exists.
	job, err := h.store.ClaimNextBuildJob()
	require.NoError(t, err)
	require.Equal(t, v.ID, job.VersionID)
	require.Equal(t, "go", job.Language)
}

func TestPublishDuplicateRejected(t *testing.T) {
	h := newHarness(t)
	body := wire.PublishRequest{RepoURL: "file:///x", CommitSHA: "abc", Collection: "gdal", Version: "1.0.0"}
	require.Equal(t, http.StatusAccepted, h.do(t, http.MethodPost, "/publish", "dev-token", body).Code)
	require.Equal(t, http.StatusConflict, h.do(t, http.MethodPost, "/publish", "dev-token", body).Code)
}

func TestListCollectionsAndVersions(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPost, "/publish", "dev-token", wire.PublishRequest{
		RepoURL: "file:///x", CommitSHA: "abc", Collection: "gdal", Version: "1.0.0", Language: "go",
	})

	w := h.do(t, http.MethodGet, "/collections", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var cols []wire.CollectionInfo
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cols))
	require.Len(t, cols, 1)
	require.Equal(t, "gdal", cols[0].Name)

	w = h.do(t, http.MethodGet, "/collections/gdal/versions", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var vers []wire.VersionInfo
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &vers))
	require.Len(t, vers, 1)
	require.Equal(t, "1.0.0", vers[0].Version)
}

func TestChangeStateAuthz(t *testing.T) {
	h := newHarness(t)
	// Set up an available version owned by user-1.
	c, _, _ := h.store.EnsureCollection("gdal", "user-1", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: store.StateAvailable}
	require.NoError(t, h.store.CreateVersion(v))

	// Owner may yank.
	w := h.do(t, http.MethodPost, "/collections/gdal/1.0.0/state", "dev-token",
		wire.StateChangeRequest{ToState: "yanked", Reason: "superseded"})
	require.Equal(t, http.StatusOK, w.Code)

	// Owner may not recall (operator-only).
	v2 := &store.Version{CollectionID: c.ID, Version: "2.0.0", State: store.StateAvailable}
	require.NoError(t, h.store.CreateVersion(v2))
	w = h.do(t, http.MethodPost, "/collections/gdal/2.0.0/state", "dev-token",
		wire.StateChangeRequest{ToState: "recalled", Reason: "cve"})
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestChangeStateRecallByOperator(t *testing.T) {
	h := newHarness(t, "user-1") // user-1 is an admin
	c, _, _ := h.store.EnsureCollection("gdal", "user-1", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: store.StateAvailable}
	require.NoError(t, h.store.CreateVersion(v))

	w := h.do(t, http.MethodPost, "/collections/gdal/1.0.0/state", "dev-token",
		wire.StateChangeRequest{ToState: "recalled", Reason: "cve"})
	require.Equal(t, http.StatusOK, w.Code)

	got, _ := h.store.GetVersion("gdal", "1.0.0")
	require.Equal(t, store.StateRecalled, got.State)
}

// seedArtifact creates an available version with a signed artifact stored in
// blob, plus a worker service token. Returns the worker token.
func (h *harness) seedArtifact(t *testing.T, stateVal store.State) string {
	t.Helper()
	c, _, _ := h.store.EnsureCollection("gdal", "user-1", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: stateVal}
	require.NoError(t, h.store.CreateVersion(v))

	data := []byte("tarball-bytes")
	sig, keyID, err := h.keyset.SignArtifact(data)
	require.NoError(t, err)
	artKey := "artifacts/gdal/1.0.0/linux/amd64.tar.gz"
	sigKey := artKey + ".sig"
	require.NoError(t, h.blob.Put(context.Background(), artKey, bytes.NewReader(data), int64(len(data)), ""))
	require.NoError(t, h.blob.Put(context.Background(), sigKey, bytes.NewReader(sig), int64(len(sig)), ""))
	require.NoError(t, h.store.CreateArtifact(&store.Artifact{
		VersionID: v.ID, Platform: "linux", Arch: "amd64",
		ContentHash: "h", ArtifactKey: artKey, SigKey: sigKey, SigningKeyID: keyID,
	}))

	require.NoError(t, h.store.CreateServiceToken(&store.ServiceToken{
		Name: "worker-1", TokenHash: auth.HashToken("worker-token"), Active: true,
	}))
	return "worker-token"
}

func TestFetchArtifactAvailable(t *testing.T) {
	h := newHarness(t)
	tok := h.seedArtifact(t, store.StateAvailable)

	w := h.do(t, http.MethodGet, "/artifacts/gdal/1.0.0/linux/amd64", tok, nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "tarball-bytes", w.Body.String())

	// Signature endpoint serves the .sig and verifies against /pubkeys.
	w = h.do(t, http.MethodGet, "/artifacts/gdal/1.0.0/linux/amd64.sig", tok, nil)
	require.Equal(t, http.StatusOK, w.Code)
	keys, err := h.keyset.PublicKeys()
	require.NoError(t, err)
	require.True(t, sign.Verify(keys, []byte("tarball-bytes"), w.Body.Bytes()))
}

func TestFetchArtifactRequiresWorkerToken(t *testing.T) {
	h := newHarness(t)
	h.seedArtifact(t, store.StateAvailable)
	w := h.do(t, http.MethodGet, "/artifacts/gdal/1.0.0/linux/amd64", "", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFetchArtifactRecalledAndYankedRefused(t *testing.T) {
	for _, tc := range []struct {
		state store.State
	}{{store.StateRecalled}, {store.StateYanked}} {
		h := newHarness(t)
		tok := h.seedArtifact(t, tc.state)
		w := h.do(t, http.MethodGet, "/artifacts/gdal/1.0.0/linux/amd64", tok, nil)
		require.Equal(t, http.StatusGone, w.Code, "state %s must refuse fetch", tc.state)
	}
}

func TestArtifactMeta(t *testing.T) {
	h := newHarness(t)
	tok := h.seedArtifact(t, store.StateAvailable)

	w := h.do(t, http.MethodGet, "/artifacts/gdal/1.0.0/linux/amd64/meta", tok, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var meta wire.ArtifactMeta
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &meta))
	require.Equal(t, "1.0.0", meta.Version)
	require.Equal(t, "linux", meta.Platform)
	require.Equal(t, "amd64", meta.Arch)
	require.Equal(t, "h", meta.ContentHash)
	require.Equal(t, "available", meta.State)

	// Unknown artifact → 404.
	w = h.do(t, http.MethodGet, "/artifacts/gdal/9.9.9/linux/amd64/meta", tok, nil)
	require.Equal(t, http.StatusNotFound, w.Code)

	// Requires a worker token.
	w = h.do(t, http.MethodGet, "/artifacts/gdal/1.0.0/linux/amd64/meta", "", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestArtifactMetaReportsRecalledState(t *testing.T) {
	// Unlike fetchArtifact (410 on recall), meta returns the state so the worker
	// can drive its recall-freshness re-check.
	h := newHarness(t)
	tok := h.seedArtifact(t, store.StateRecalled)
	w := h.do(t, http.MethodGet, "/artifacts/gdal/1.0.0/linux/amd64/meta", tok, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var meta wire.ArtifactMeta
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &meta))
	require.Equal(t, "recalled", meta.State)
}

func TestPubkeys(t *testing.T) {
	h := newHarness(t)
	w := h.do(t, http.MethodGet, "/pubkeys", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp wire.PubKeysResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Keys, 1)
}

func TestHealthz(t *testing.T) {
	h := newHarness(t)
	w := h.do(t, http.MethodGet, "/healthz", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
}
