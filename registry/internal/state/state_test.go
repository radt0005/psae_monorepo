package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"spade_registry/internal/audit"
	"spade_registry/internal/mirror"
	"spade_registry/internal/store"
)

func TestCanTransitionHappyPath(t *testing.T) {
	chain := []store.State{
		store.StateSubmitted, store.StateScreening, store.StateScreened,
		store.StateBuilding, store.StateAvailable,
	}
	for i := 0; i < len(chain)-1; i++ {
		require.True(t, CanTransition(chain[i], chain[i+1]), "%s→%s", chain[i], chain[i+1])
	}
}

func TestCanTransitionOffStates(t *testing.T) {
	require.True(t, CanTransition(store.StateAvailable, store.StateDeprecated))
	require.True(t, CanTransition(store.StateAvailable, store.StateYanked))
	require.True(t, CanTransition(store.StateDeprecated, store.StateYanked))
	require.False(t, CanTransition(store.StateYanked, store.StateAvailable), "no un-yank")
	require.False(t, CanTransition(store.StateAvailable, store.StateBuilding), "no backward")
}

func TestRecallFromAnyStateButIrreversible(t *testing.T) {
	for _, from := range []store.State{
		store.StateSubmitted, store.StateScreening, store.StateScreened,
		store.StateBuilding, store.StateAvailable, store.StateDeprecated,
		store.StateYanked, store.StateFailed,
	} {
		require.True(t, CanTransition(from, store.StateRecalled), "recall from %s", from)
	}
	require.False(t, CanTransition(store.StateRecalled, store.StateRecalled), "recall is terminal")
	require.False(t, CanTransition(store.StateRecalled, store.StateAvailable), "recall irreversible")
}

func TestAuthorize(t *testing.T) {
	owner := Actor{ID: "u", Type: audit.ActorDeveloper, IsOwner: true}
	stranger := Actor{ID: "x", Type: audit.ActorDeveloper}
	operator := Actor{ID: "op", Type: audit.ActorOperator, IsOperator: true}
	system := Actor{ID: "sys", Type: audit.ActorSystem}

	// Deprecate/yank: owner yes, stranger no.
	require.NoError(t, Authorize(owner, store.StateYanked))
	require.ErrorIs(t, Authorize(stranger, store.StateYanked), ErrUnauthorized)

	// Recall: operator only.
	require.NoError(t, Authorize(operator, store.StateRecalled))
	require.ErrorIs(t, Authorize(owner, store.StateRecalled), ErrUnauthorized)

	// Pipeline edges: system (or operator) only.
	require.NoError(t, Authorize(system, store.StateAvailable))
	require.ErrorIs(t, Authorize(owner, store.StateBuilding), ErrUnauthorized)
}

// recordingMirror captures mirror calls for assertions.
type recordingMirror struct {
	upserts int
	removes int
}

func (m *recordingMirror) UpsertVersion(*store.Version, string, []store.BlockMeta) error {
	m.upserts++
	return nil
}
func (m *recordingMirror) RemoveVersion([]store.BlockMeta) error {
	m.removes++
	return nil
}

func newMachine(t *testing.T) (*Machine, *store.Store, *recordingMirror) {
	t.Helper()
	st, err := store.OpenSQLite(t.TempDir() + "/state.db")
	require.NoError(t, err)
	rm := &recordingMirror{}
	return New(st, audit.New(st), rm), st, rm
}

func TestTransitionAppliesAuditAndMirror(t *testing.T) {
	m, st, rm := newMachine(t)
	c, _, _ := st.EnsureCollection("gdal", "u", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: store.StateBuilding}
	require.NoError(t, st.CreateVersion(v))
	require.NoError(t, st.ReplaceBlockMeta(v.ID, []store.BlockMeta{{BlockID: "gdal.a", Name: "a"}}))

	sys := Actor{ID: "sys", Type: audit.ActorSystem}
	require.NoError(t, m.Transition(sys, v, "gdal", store.StateAvailable, "build complete", ""))
	require.Equal(t, store.StateAvailable, v.State)
	require.Equal(t, 1, rm.upserts, "available upserts into mirror")

	// Audit recorded.
	entries, err := st.ListAuditEntries(10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "transition", entries[0].EventType)

	// Owner yanks → mirror removal.
	owner := Actor{ID: "u", Type: audit.ActorDeveloper, IsOwner: true}
	require.NoError(t, m.Transition(owner, v, "gdal", store.StateYanked, "superseded", ""))
	require.Equal(t, 1, rm.removes)
}

func TestTransitionRejectsIllegalAndUnauthorized(t *testing.T) {
	m, st, _ := newMachine(t)
	c, _, _ := st.EnsureCollection("gdal", "u", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: store.StateAvailable}
	require.NoError(t, st.CreateVersion(v))

	sys := Actor{ID: "sys", Type: audit.ActorSystem}
	require.ErrorIs(t, m.Transition(sys, v, "gdal", store.StateBuilding, "", ""), ErrIllegalTransition)

	stranger := Actor{ID: "x", Type: audit.ActorDeveloper}
	require.ErrorIs(t, m.Transition(stranger, v, "gdal", store.StateYanked, "", ""), ErrUnauthorized)
}

func TestNewWithNilMirrorUsesNoop(t *testing.T) {
	st, _ := store.OpenSQLite(t.TempDir() + "/n.db")
	m := New(st, audit.New(st), nil)
	require.IsType(t, mirror.NoopMirror{}, m.mirror)
}
