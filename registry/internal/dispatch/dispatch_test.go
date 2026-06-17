package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"spade_registry/internal/audit"
	"spade_registry/internal/config"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
)

func newDispatcher(t *testing.T, l Launcher) (*Dispatcher, *store.Store) {
	t.Helper()
	st, err := store.OpenSQLite(t.TempDir() + "/dispatch.db")
	require.NoError(t, err)
	d := New(Options{
		Config:      config.RegistryConfig{StagingPrefix: "staging/", BuilderImages: map[string]string{"go": "img"}},
		Store:       st,
		State:       state.New(st, audit.New(st), nil),
		Launcher:    l,
		RegistryURL: "http://registry:8080",
	})
	return d, st
}

func seedQueuedJob(t *testing.T, st *store.Store) *store.BuildJob {
	t.Helper()
	c, _, _ := st.EnsureCollection("gdal", "u", "go")
	v := &store.Version{CollectionID: c.ID, Version: "1.0.0", State: store.StateSubmitted}
	require.NoError(t, st.CreateVersion(v))
	job := &store.BuildJob{VersionID: v.ID, Language: "go", State: store.BuildQueued}
	require.NoError(t, st.CreateBuildJob(job))
	return job
}

// captureLauncher records the env it was launched with.
type captureLauncher struct {
	env   map[string]string
	image string
	err   error
}

func (c *captureLauncher) Run(ctx context.Context, image string, env map[string]string) error {
	c.image = image
	c.env = env
	return c.err
}

func TestProcessOneClaimsAndLaunches(t *testing.T) {
	cl := &captureLauncher{}
	d, st := newDispatcher(t, cl)
	job := seedQueuedJob(t, st)

	worked, err := d.ProcessOne(context.Background())
	require.NoError(t, err)
	require.True(t, worked)

	// Version advanced to screening; build container was launched with the
	// right image and a token but NO database credentials.
	v, _ := st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateScreening, v.State)
	require.Equal(t, "img", cl.image)
	require.Equal(t, job.ID, cl.env["BUILD_JOB_ID"])
	require.NotEmpty(t, cl.env["BUILD_TOKEN"])
	require.NotContains(t, cl.env, "DATABASE_URL")

	// The job's token hash was persisted (so BuilderAuth can validate it).
	fresh, _ := st.GetBuildJob(job.ID)
	require.NotEmpty(t, fresh.TokenHash)
	require.Equal(t, store.BuildRunning, fresh.State)
}

func TestProcessOneEmptyQueue(t *testing.T) {
	d, _ := newDispatcher(t, &captureLauncher{})
	worked, err := d.ProcessOne(context.Background())
	require.NoError(t, err)
	require.False(t, worked)
}

func TestProcessOneContainerFailureMarksFailed(t *testing.T) {
	cl := &captureLauncher{err: errors.New("boom")}
	d, st := newDispatcher(t, cl)
	job := seedQueuedJob(t, st)

	worked, err := d.ProcessOne(context.Background())
	require.True(t, worked)
	require.Error(t, err)

	// The builder never reported, so the dispatcher fails the job + version.
	v, _ := st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateFailed, v.State)
	fresh, _ := st.GetBuildJob(job.ID)
	require.Equal(t, store.BuildFailed, fresh.State)
}
