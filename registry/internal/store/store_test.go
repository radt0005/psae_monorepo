package store

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	// File-backed temp DB avoids the shared-cache quirks of :memory: under
	// concurrent connections while staying isolated per test.
	s, err := OpenSQLite(t.TempDir() + "/registry.db")
	require.NoError(t, err)
	return s
}

func TestEnsureCollectionIdempotent(t *testing.T) {
	s := newTestStore(t)

	c1, created, err := s.EnsureCollection("gdal", "user-1", "go")
	require.NoError(t, err)
	require.True(t, created)

	c2, created, err := s.EnsureCollection("gdal", "user-2", "go")
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, c1.ID, c2.ID)
	require.Equal(t, "user-1", c2.OwnerUserID, "owner is not reassigned")
}

func TestVersionUniqueConstraint(t *testing.T) {
	s := newTestStore(t)
	c, _, err := s.EnsureCollection("gdal", "user-1", "go")
	require.NoError(t, err)

	require.NoError(t, s.CreateVersion(&Version{CollectionID: c.ID, Version: "1.0.0", State: StateSubmitted}))
	err = s.CreateVersion(&Version{CollectionID: c.ID, Version: "1.0.0", State: StateSubmitted})
	require.Error(t, err, "duplicate (collection, version) must be rejected")
}

func TestSetVersionStateAndLookup(t *testing.T) {
	s := newTestStore(t)
	c, _, _ := s.EnsureCollection("gdal", "u", "go")
	v := &Version{CollectionID: c.ID, Version: "1.0.0", State: StateSubmitted}
	require.NoError(t, s.CreateVersion(v))

	require.NoError(t, s.SetVersionState(v.ID, StateAvailable, ""))
	got, err := s.GetVersion("gdal", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, StateAvailable, got.State)
}

func TestGetArtifactStateGating(t *testing.T) {
	s := newTestStore(t)
	c, _, _ := s.EnsureCollection("gdal", "u", "go")
	v := &Version{CollectionID: c.ID, Version: "1.0.0", State: StateAvailable}
	require.NoError(t, s.CreateVersion(v))
	require.NoError(t, s.CreateArtifact(&Artifact{
		VersionID: v.ID, Platform: "linux", Arch: "amd64",
		ContentHash: "abc", ArtifactKey: "k", SigKey: "k.sig",
	}))

	a, vv, err := s.GetArtifact("gdal", "1.0.0", "linux", "amd64")
	require.NoError(t, err)
	require.Equal(t, "abc", a.ContentHash)
	require.Equal(t, StateAvailable, vv.State)

	_, _, err = s.GetArtifact("gdal", "1.0.0", "linux", "arm64")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestClaimNextBuildJobSingleClaimUnderConcurrency(t *testing.T) {
	s := newTestStore(t)
	c, _, _ := s.EnsureCollection("gdal", "u", "go")
	v := &Version{CollectionID: c.ID, Version: "1.0.0", State: StateSubmitted}
	require.NoError(t, s.CreateVersion(v))
	require.NoError(t, s.CreateBuildJob(&BuildJob{VersionID: v.ID, Language: "go", State: BuildQueued}))

	const workers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claims  int
		errored int
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, err := s.ClaimNextBuildJob()
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && j != nil:
				claims++
			case err == ErrNotFound:
				// queue empty for this racer — fine
			default:
				errored++
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 0, errored)
	require.Equal(t, 1, claims, "exactly one worker may claim the single job")
}

func TestReplaceBlockMetaIdempotent(t *testing.T) {
	s := newTestStore(t)
	c, _, _ := s.EnsureCollection("gdal", "u", "go")
	v := &Version{CollectionID: c.ID, Version: "1.0.0", State: StateBuilding}
	require.NoError(t, s.CreateVersion(v))

	require.NoError(t, s.ReplaceBlockMeta(v.ID, []BlockMeta{{BlockID: "gdal.a", Name: "a"}}))
	require.NoError(t, s.ReplaceBlockMeta(v.ID, []BlockMeta{
		{BlockID: "gdal.a", Name: "a"}, {BlockID: "gdal.b", Name: "b"},
	}))
	got, err := s.ListBlockMeta(v.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestSigningKeyLifecycle(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateSigningKey(&SigningKey{ID: "k1", PublicKey: "p1", Active: true, Listed: true}))

	active, err := s.ActiveSigningKey()
	require.NoError(t, err)
	require.Equal(t, "k1", active.ID)

	// Rotate: demote old, add new active.
	require.NoError(t, s.DeactivateSigningKeys())
	require.NoError(t, s.CreateSigningKey(&SigningKey{ID: "k2", PublicKey: "p2", Active: true, Listed: true}))

	active, err = s.ActiveSigningKey()
	require.NoError(t, err)
	require.Equal(t, "k2", active.ID)

	listed, err := s.ListedSigningKeys()
	require.NoError(t, err)
	require.Len(t, listed, 2, "old key still served by /pubkeys until retired")

	require.NoError(t, s.RetireSigningKey("k1"))
	listed, err = s.ListedSigningKeys()
	require.NoError(t, err)
	require.Len(t, listed, 1)
}

func TestServiceTokenLookup(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateServiceToken(&ServiceToken{Name: "w1", TokenHash: "h1", Active: true}))

	tok, err := s.ActiveServiceTokenByHash("h1")
	require.NoError(t, err)
	require.Equal(t, "w1", tok.Name)

	_, err = s.ActiveServiceTokenByHash("nope")
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, s.DeactivateServiceToken(tok.ID))
	_, err = s.ActiveServiceTokenByHash("h1")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestAuditAppend(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateAuditEntry(&AuditEntry{EventType: "publish", ActorID: "u1"}))
	require.NoError(t, s.CreateAuditEntry(&AuditEntry{EventType: "transition", ActorID: "u1"}))
	es, err := s.ListAuditEntries(10)
	require.NoError(t, err)
	require.Len(t, es, 2)
}
