package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"spade_registry/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenSQLite(t.TempDir() + "/auth.db")
	require.NoError(t, err)
	return s
}

func newSessionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/session.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.Table("session").AutoMigrate(&sessionRow{}))
	return db
}

func TestSessionVerifier(t *testing.T) {
	db := newSessionDB(t)
	require.NoError(t, db.Table("session").Create(&sessionRow{
		ID: "s1", UserID: "user-1", Token: "good", ExpiresAt: time.Now().Add(time.Hour),
	}).Error)
	require.NoError(t, db.Table("session").Create(&sessionRow{
		ID: "s2", UserID: "user-2", Token: "expired", ExpiresAt: time.Now().Add(-time.Hour),
	}).Error)

	v := NewSessionVerifier(db)

	dev, err := v.Verify("good")
	require.NoError(t, err)
	require.Equal(t, "user-1", dev.UserID)

	_, err = v.Verify("expired")
	require.ErrorIs(t, err, ErrUnauthenticated)
	_, err = v.Verify("missing")
	require.ErrorIs(t, err, ErrUnauthenticated)
	_, err = v.Verify("")
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestWorkerAuth(t *testing.T) {
	st := newStore(t)
	require.NoError(t, st.CreateServiceToken(&store.ServiceToken{
		Name: "worker-a", TokenHash: HashToken("secret-token"), Active: true,
	}))

	wa := NewWorkerAuth(st)
	w, err := wa.Verify("secret-token")
	require.NoError(t, err)
	require.Equal(t, "worker-a", w.Name)

	_, err = wa.Verify("wrong")
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestWorkerAuthRotation(t *testing.T) {
	st := newStore(t)
	// Two active tokens coexist during rotation so a worker mid-rotation never
	// drops work (registry.md §7.2).
	require.NoError(t, st.CreateServiceToken(&store.ServiceToken{Name: "w", TokenHash: HashToken("old"), Active: true}))
	require.NoError(t, st.CreateServiceToken(&store.ServiceToken{Name: "w", TokenHash: HashToken("new"), Active: true}))
	wa := NewWorkerAuth(st)
	_, err := wa.Verify("old")
	require.NoError(t, err)
	_, err = wa.Verify("new")
	require.NoError(t, err)
}

func TestBuilderAuth(t *testing.T) {
	st := newStore(t)
	c, _, _ := st.EnsureCollection("gdal", "u", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: store.StateScreening}
	require.NoError(t, st.CreateVersion(v))
	job := &store.BuildJob{VersionID: v.ID, Language: "go", State: store.BuildRunning, TokenHash: HashToken("job-token")}
	require.NoError(t, st.CreateBuildJob(job))

	ba := NewBuilderAuth(st)
	got, err := ba.Verify(job.ID, "job-token")
	require.NoError(t, err)
	require.Equal(t, job.ID, got.ID)

	_, err = ba.Verify(job.ID, "wrong")
	require.ErrorIs(t, err, ErrUnauthenticated)
	_, err = ba.Verify("no-such-job", "job-token")
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestBuilderAuthRejectsClosedJob(t *testing.T) {
	st := newStore(t)
	c, _, _ := st.EnsureCollection("gdal", "u", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: store.StateAvailable}
	require.NoError(t, st.CreateVersion(v))
	job := &store.BuildJob{VersionID: v.ID, State: store.BuildSucceeded, TokenHash: HashToken("t")}
	require.NoError(t, st.CreateBuildJob(job))

	_, err := NewBuilderAuth(st).Verify(job.ID, "t")
	require.ErrorIs(t, err, ErrUnauthenticated)
}
