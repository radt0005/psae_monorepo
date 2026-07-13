package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

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

// backdateJob makes a job look untouched for hours, past any stale window.
func backdateJob(t *testing.T, st *store.Store, id string) {
	t.Helper()
	past := time.Now().Add(-2 * time.Hour)
	require.NoError(t, st.DB().Model(&store.BuildJob{}).Where("id = ?", id).
		UpdateColumn("updated_at", past).Error)
}

// claimAndStrand simulates a dispatcher that claimed the job, moved the version
// into screening, and died before the build reported a terminal state.
func claimAndStrand(t *testing.T, d *Dispatcher, st *store.Store, job *store.BuildJob) {
	t.Helper()
	_, err := st.ClaimNextBuildJob()
	require.NoError(t, err)
	require.NoError(t, st.SetBuildJobState(job.ID, store.BuildRunning, ""))
	v, err := st.GetVersionByID(job.VersionID)
	require.NoError(t, err)
	require.NoError(t, d.state.Transition(systemActor, v, "gdal", store.StateScreening, "dispatch", ""))
	backdateJob(t, st, job.ID)
}

func TestReapRequeuesAbandonedJob(t *testing.T) {
	cl := &captureLauncher{}
	d, st := newDispatcher(t, cl)
	job := seedQueuedJob(t, st)
	claimAndStrand(t, d, st, job)

	require.NoError(t, d.Reap())

	fresh, _ := st.GetBuildJob(job.ID)
	require.Equal(t, store.BuildQueued, fresh.State)
	require.Empty(t, fresh.TokenHash)
	v, _ := st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateSubmitted, v.State)

	// The requeued job dispatches cleanly on the next pass.
	worked, err := d.ProcessOne(context.Background())
	require.NoError(t, err)
	require.True(t, worked)
	v, _ = st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateScreening, v.State)
}

func TestReapRequeuesJobWithVersionStillSubmitted(t *testing.T) {
	// Crash between claiming and the screening transition: no version rollback
	// is needed, just a requeue.
	d, st := newDispatcher(t, &captureLauncher{})
	job := seedQueuedJob(t, st)
	_, err := st.ClaimNextBuildJob()
	require.NoError(t, err)
	backdateJob(t, st, job.ID)

	require.NoError(t, d.Reap())

	fresh, _ := st.GetBuildJob(job.ID)
	require.Equal(t, store.BuildQueued, fresh.State)
	v, _ := st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateSubmitted, v.State)
}

func TestReapAbandonsJobAfterMaxAttempts(t *testing.T) {
	d, st := newDispatcher(t, &captureLauncher{})
	job := seedQueuedJob(t, st)
	claimAndStrand(t, d, st, job)
	require.NoError(t, st.DB().Model(&store.BuildJob{}).Where("id = ?", job.ID).
		UpdateColumn("attempts", d.MaxAttempts).Error)

	require.NoError(t, d.Reap())

	fresh, _ := st.GetBuildJob(job.ID)
	require.Equal(t, store.BuildFailed, fresh.State)
	v, _ := st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateFailed, v.State)
}

func TestReapLeavesFreshJobsAlone(t *testing.T) {
	d, st := newDispatcher(t, &captureLauncher{})
	job := seedQueuedJob(t, st)
	// Claimed and running right now — a live build, not a stuck one.
	_, err := st.ClaimNextBuildJob()
	require.NoError(t, err)
	require.NoError(t, st.SetBuildJobState(job.ID, store.BuildRunning, ""))

	require.NoError(t, d.Reap())

	fresh, _ := st.GetBuildJob(job.ID)
	require.Equal(t, store.BuildRunning, fresh.State)
}

func TestReapLeavesApprovalGateAlone(t *testing.T) {
	// REQUIRE_APPROVAL holds a version at screened while its job row stays
	// running; the reaper must not requeue or fail it.
	d, st := newDispatcher(t, &captureLauncher{})
	job := seedQueuedJob(t, st)
	claimAndStrand(t, d, st, job)
	v, _ := st.GetVersionByID(job.VersionID)
	require.NoError(t, d.state.Transition(systemActor, v, "gdal", store.StateScreened, "screening passed", ""))

	require.NoError(t, d.Reap())

	fresh, _ := st.GetBuildJob(job.ID)
	require.Equal(t, store.BuildRunning, fresh.State)
	v, _ = st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateScreened, v.State)
}

func TestReapMarksJobSucceededWhenVersionAvailable(t *testing.T) {
	// Crash after the build completed (version promoted) but before the job row
	// got its terminal update: close out the job, don't rebuild.
	d, st := newDispatcher(t, &captureLauncher{})
	job := seedQueuedJob(t, st)
	claimAndStrand(t, d, st, job)
	v, _ := st.GetVersionByID(job.VersionID)
	require.NoError(t, d.state.Transition(systemActor, v, "gdal", store.StateScreened, "", ""))
	require.NoError(t, d.state.Transition(systemActor, v, "gdal", store.StateBuilding, "", ""))
	require.NoError(t, d.state.Transition(systemActor, v, "gdal", store.StateAvailable, "", ""))

	require.NoError(t, d.Reap())

	fresh, _ := st.GetBuildJob(job.ID)
	require.Equal(t, store.BuildSucceeded, fresh.State)
	v, _ = st.GetVersionByID(job.VersionID)
	require.Equal(t, store.StateAvailable, v.State)
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
