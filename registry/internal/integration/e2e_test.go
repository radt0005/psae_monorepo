// Package integration exercises the full registry trust chain end to end:
// publish → dispatch → (in-process) build → screen → sign → store → fetch →
// verify, plus the recall path. It uses a filesystem blob store and SQLite, an
// in-process launcher (no Docker), and a real Go collection git fixture, so the
// whole chain runs offline.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"spade_registry/internal/api"
	"spade_registry/internal/audit"
	"spade_registry/internal/auth"
	"spade_registry/internal/blob"
	"spade_registry/internal/builder"
	"spade_registry/internal/config"
	"spade_registry/internal/dispatch"
	"spade_registry/internal/sign"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
	"spade_registry/internal/testutil"
	"spade_registry/internal/wire"
)

// fakeDev authorizes a single developer token.
type fakeDev struct{ token, user string }

func (f fakeDev) Verify(tok string) (auth.Developer, error) {
	if tok == f.token && tok != "" {
		return auth.Developer{UserID: f.user}, nil
	}
	return auth.Developer{}, auth.ErrUnauthenticated
}

type stack struct {
	url    string
	dbPath string
	store  *store.Store
	blob   blob.Store
	keyset *sign.Keyset
	disp   *dispatch.Dispatcher
}

func newStack(t *testing.T, admins ...string) *stack {
	t.Helper()
	dbPath := t.TempDir() + "/e2e.db"
	st, err := store.OpenSQLite(dbPath)
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
		BuilderImages:  map[string]string{"go": "spade-builder-go"},
	}
	srv := api.New(api.Options{
		Config: cfg, Store: st, Keyset: ks, Audit: audit.New(st),
		State: state.New(st, audit.New(st), nil), Blob: bs,
		Dev: fakeDev{token: "dev-token", user: "user-1"},
	})
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)

	// In-process launcher mirrors what the container's builder binary does,
	// reaching the registry over HTTP and sharing the same blob store.
	runner := func(ctx context.Context, env map[string]string) error {
		c := builder.NewClient(env["REGISTRY_URL"], env["BUILD_JOB_ID"], env["BUILD_TOKEN"])
		return builder.Run(ctx, builder.Deps{
			Client:   c,
			Cloner:   builder.GitCloner{},
			Screener: builder.NoopScreener{},
			Blob:     bs,
		})
	}
	disp := dispatch.New(dispatch.Options{
		Config: cfg, Store: st, State: state.New(st, audit.New(st), nil),
		Launcher:    dispatch.InProcessLauncher{Runner: runner},
		RegistryURL: ts.URL,
	})

	return &stack{url: ts.URL, dbPath: dbPath, store: st, blob: bs, keyset: ks, disp: disp}
}

func (s *stack) publish(t *testing.T, col testutil.GoCollection) {
	t.Helper()
	body, _ := json.Marshal(wire.PublishRequest{
		RepoURL: col.RepoURL, CommitSHA: col.CommitSHA,
		Collection: col.Collection, Version: col.Version, Language: "go",
	})
	req, _ := http.NewRequest(http.MethodPost, s.url+"/publish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func (s *stack) get(t *testing.T, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, s.url+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestEndToEndPublishBuildFetchVerify(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	s := newStack(t)
	col := testutil.NewGoCollectionRepo(t)

	// 1. Publish.
	s.publish(t, col)

	// 2. Dispatch drives the in-process build to completion.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	worked, err := s.disp.ProcessOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)

	// 3. The version is now available.
	v, err := s.store.GetVersion(col.Collection, col.Version)
	require.NoError(t, err)
	require.Equal(t, store.StateAvailable, v.State, "version error: %s", v.Error)

	// A service token for the worker.
	require.NoError(t, s.store.CreateServiceToken(&store.ServiceToken{
		Name: "worker-1", TokenHash: auth.HashToken("worker-token"), Active: true,
	}))

	// 4. Worker fetches the tarball and signature.
	resp := s.get(t, "/artifacts/"+col.Collection+"/"+col.Version+"/linux/amd64", "worker-token")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	tarball := readAll(t, resp)

	resp = s.get(t, "/artifacts/"+col.Collection+"/"+col.Version+"/linux/amd64.sig", "worker-token")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	sigBytes := readAll(t, resp)

	// 5. Verify the signature against /pubkeys (the worker's verify step) and
	//    confirm the content hash matches registry metadata.
	resp = s.get(t, "/pubkeys", "worker-token")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var pk wire.PubKeysResponse
	require.NoError(t, json.Unmarshal(readAll(t, resp), &pk))
	require.True(t, sign.Verify(pk.Keys, tarball, sigBytes), "artifact signature verifies")

	art, _, err := s.store.GetArtifact(col.Collection, col.Version, "linux", "amd64")
	require.NoError(t, err)
	gotHash, err := builder.HashReader(bytes.NewReader(tarball))
	require.NoError(t, err)
	require.Equal(t, art.ContentHash, gotHash, "fetched bytes match recorded content hash")

	// Block metadata was captured for the mirror.
	blocks, err := s.store.ListBlockMeta(v.ID)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, "hello.greet", blocks[0].BlockID)
}

// TestEndToEndSplitBuildService mirrors the production split (hosting.md §5):
// registryd runs control-plane-only (BUILD_DISPATCH_ENABLED=false) while a
// standalone build service — buildrunnerd — claims the same queue over its own
// database connection and drives the build. The control plane's dispatcher is
// never used; only the runner's is.
func TestEndToEndSplitBuildService(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	s := newStack(t)
	col := testutil.NewGoCollectionRepo(t)
	s.publish(t, col)

	// The build runner's side: a second store connection to the same database
	// and its own state machine, wired exactly like cmd/buildrunnerd.
	runnerStore, err := store.OpenSQLite(s.dbPath)
	require.NoError(t, err)
	runner := func(ctx context.Context, env map[string]string) error {
		c := builder.NewClient(env["REGISTRY_URL"], env["BUILD_JOB_ID"], env["BUILD_TOKEN"])
		return builder.Run(ctx, builder.Deps{
			Client:   c,
			Cloner:   builder.GitCloner{},
			Screener: builder.NoopScreener{},
			Blob:     s.blob,
		})
	}
	runnerDisp := dispatch.New(dispatch.Options{
		Config: config.RegistryConfig{
			StagingPrefix:  "staging/",
			ArtifactPrefix: "artifacts/",
			BuilderImages:  map[string]string{"go": "spade-builder-go"},
		},
		Store:       runnerStore,
		State:       state.New(runnerStore, audit.New(runnerStore), nil),
		Launcher:    dispatch.InProcessLauncher{Runner: runner},
		RegistryURL: s.url,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	worked, err := runnerDisp.ProcessOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)

	// The control plane sees the version as available and serves the artifact.
	v, err := s.store.GetVersion(col.Collection, col.Version)
	require.NoError(t, err)
	require.Equal(t, store.StateAvailable, v.State, "version error: %s", v.Error)

	require.NoError(t, s.store.CreateServiceToken(&store.ServiceToken{
		Name: "worker-1", TokenHash: auth.HashToken("worker-token"), Active: true,
	}))
	resp := s.get(t, "/artifacts/"+col.Collection+"/"+col.Version+"/linux/amd64", "worker-token")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	readAll(t, resp)
}

func TestEndToEndRecallRefusesFetch(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	s := newStack(t, "user-1") // user-1 is an operator
	col := testutil.NewGoCollectionRepo(t)
	s.publish(t, col)

	_, err := s.disp.ProcessOne(context.Background())
	require.NoError(t, err)

	require.NoError(t, s.store.CreateServiceToken(&store.ServiceToken{
		Name: "worker-1", TokenHash: auth.HashToken("worker-token"), Active: true,
	}))

	// Recall via the API as the operator.
	body, _ := json.Marshal(wire.StateChangeRequest{ToState: "recalled", Reason: "cve-2026"})
	req, _ := http.NewRequest(http.MethodPost,
		s.url+"/collections/"+col.Collection+"/"+col.Version+"/state", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// versions endpoint reports recalled (the signal the worker acts on).
	resp = s.get(t, "/collections/"+col.Collection+"/versions", "worker-token")
	var vers []wire.VersionInfo
	require.NoError(t, json.Unmarshal(readAll(t, resp), &vers))
	require.Equal(t, string(store.StateRecalled), vers[0].State)

	// Fetch is refused.
	resp = s.get(t, "/artifacts/"+col.Collection+"/"+col.Version+"/linux/amd64", "worker-token")
	require.Equal(t, http.StatusGone, resp.StatusCode)
}

func readAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, err := buf.ReadFrom(resp.Body)
	require.NoError(t, err)
	return buf.Bytes()
}
