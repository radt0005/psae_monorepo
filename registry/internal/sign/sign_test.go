package sign

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"spade_registry/internal/store"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	kp, err := GenerateKeypair()
	require.NoError(t, err)

	data := []byte("artifact-bytes")
	sig, err := Sign(kp.Private, data)
	require.NoError(t, err)

	require.True(t, Verify([]string{kp.Public}, data, sig))
	require.False(t, Verify([]string{kp.Public}, []byte("tampered"), sig), "tampered data fails")
}

func TestVerifyWrongKeyFails(t *testing.T) {
	a, _ := GenerateKeypair()
	b, _ := GenerateKeypair()
	sig, _ := Sign(a.Private, []byte("x"))
	require.False(t, Verify([]string{b.Public}, []byte("x"), sig))
}

func TestVerifyMultiKeyAcceptsBoth(t *testing.T) {
	oldKey, _ := GenerateKeypair()
	newKey, _ := GenerateKeypair()
	data := []byte("payload")

	sigOld, _ := Sign(oldKey.Private, data)
	sigNew, _ := Sign(newKey.Private, data)

	trusted := []string{oldKey.Public, newKey.Public}
	require.True(t, Verify(trusted, data, sigOld), "artifact signed by old key still verifies")
	require.True(t, Verify(trusted, data, sigNew), "artifact signed by new key verifies")
}

func TestSignReader(t *testing.T) {
	kp, _ := GenerateKeypair()
	data := []byte("streamed-artifact")
	sig, err := SignReader(kp.Private, bytes.NewReader(data))
	require.NoError(t, err)
	require.True(t, Verify([]string{kp.Public}, data, sig))
}

func newKeysetStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenSQLite(t.TempDir() + "/keys.db")
	require.NoError(t, err)
	return s
}

func TestKeysetEnsureAndSign(t *testing.T) {
	ks := NewKeyset(newKeysetStore(t))
	id, err := ks.EnsureActiveKey()
	require.NoError(t, err)
	require.NotEmpty(t, id)

	// Idempotent: a second Ensure does not rotate.
	id2, err := ks.EnsureActiveKey()
	require.NoError(t, err)
	require.Equal(t, id, id2)

	sig, keyID, err := ks.SignArtifact([]byte("data"))
	require.NoError(t, err)
	require.Equal(t, id, keyID)

	pubs, err := ks.PublicKeys()
	require.NoError(t, err)
	require.True(t, Verify(pubs, []byte("data"), sig))
}

func TestKeysetRotationFlagDayFree(t *testing.T) {
	ks := NewKeyset(newKeysetStore(t))
	oldID, err := ks.EnsureActiveKey()
	require.NoError(t, err)

	// Sign an artifact with the old key.
	data := []byte("old-artifact")
	oldSig, _, err := ks.SignArtifact(data)
	require.NoError(t, err)

	// Rotate in a new active key.
	newID, err := ks.AddKey()
	require.NoError(t, err)
	require.NotEqual(t, oldID, newID)

	// New signatures use the new key; both keys are still served.
	newSig, keyID, err := ks.SignArtifact([]byte("new-artifact"))
	require.NoError(t, err)
	require.Equal(t, newID, keyID)

	pubs, err := ks.PublicKeys()
	require.NoError(t, err)
	require.Len(t, pubs, 2)
	require.True(t, Verify(pubs, data, oldSig), "old artifact still verifies during overlap")
	require.True(t, Verify(pubs, []byte("new-artifact"), newSig))

	// Retire the old key: it drops from /pubkeys.
	require.NoError(t, ks.RetireKey(oldID))
	pubs, err = ks.PublicKeys()
	require.NoError(t, err)
	require.Len(t, pubs, 1)
	require.False(t, Verify(pubs, data, oldSig), "old artifact no longer verifies after retirement")
}

func TestSignArtifactNoActiveKey(t *testing.T) {
	ks := NewKeyset(newKeysetStore(t))
	_, _, err := ks.SignArtifact([]byte("x"))
	require.ErrorIs(t, err, ErrNoActiveKey)
}
