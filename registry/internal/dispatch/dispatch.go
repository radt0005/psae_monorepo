package dispatch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"spade_registry/internal/audit"
	"spade_registry/internal/auth"
	"spade_registry/internal/config"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
)

// systemActor drives the build-pipeline state transitions.
var systemActor = state.Actor{ID: "build-dispatcher", Type: audit.ActorSystem}

// Dispatcher claims queued build jobs and launches build containers.
type Dispatcher struct {
	cfg         config.RegistryConfig
	store       *store.Store
	state       *state.Machine
	launcher    Launcher
	registryURL string // URL the build container uses to reach the registry
	log         *slog.Logger

	// PollInterval is how often the queue is polled when idle.
	PollInterval time.Duration
	// ReapInterval is how often Run sweeps for stuck jobs (see Reap).
	ReapInterval time.Duration
	// MaxAttempts is how many claims a job gets before the reaper abandons it.
	MaxAttempts int
}

// Options configures a Dispatcher.
type Options struct {
	Config      config.RegistryConfig
	Store       *store.Store
	State       *state.Machine
	Launcher    Launcher
	RegistryURL string
	Logger      *slog.Logger
}

// New builds a Dispatcher.
func New(o Options) *Dispatcher {
	log := o.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Dispatcher{
		cfg:          o.Config,
		store:        o.Store,
		state:        o.State,
		launcher:     o.Launcher,
		registryURL:  o.RegistryURL,
		log:          log,
		PollInterval: 2 * time.Second,
		ReapInterval: time.Minute,
		MaxAttempts:  3,
	}
}

// Run polls the queue until ctx is cancelled, processing one job at a time
// (single-concurrency at launch, per hosting.md §5.3), and periodically reaps
// jobs stranded by a dispatcher that died mid-build.
func (d *Dispatcher) Run(ctx context.Context) {
	t := time.NewTimer(0)
	defer t.Stop()
	var lastReap time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if time.Since(lastReap) >= d.ReapInterval {
				if err := d.Reap(); err != nil {
					d.log.Error("reap error", "err", err)
				}
				lastReap = time.Now()
			}
			worked, err := d.ProcessOne(ctx)
			if err != nil {
				d.log.Error("dispatch error", "err", err)
			}
			// Poll quickly while there is work; back off when idle.
			if worked {
				t.Reset(0)
			} else {
				t.Reset(d.PollInterval)
			}
		}
	}
}

// ProcessOne claims and runs at most one build job. It returns whether a job was
// processed.
func (d *Dispatcher) ProcessOne(ctx context.Context) (bool, error) {
	job, err := d.store.ClaimNextBuildJob()
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	v, collection, err := d.jobContext(job)
	if err != nil {
		return true, err
	}

	// submitted → screening (the container performs the actual screen+build).
	if err := d.state.Transition(systemActor, v, collection, store.StateScreening, "dispatch", ""); err != nil {
		_ = d.store.SetBuildJobState(job.ID, store.BuildFailed, "")
		return true, err
	}

	// Mint a short-lived per-job builder token; the container authenticates
	// with it and never receives database credentials.
	token, err := randomToken()
	if err != nil {
		return true, err
	}
	if err := d.store.SetBuildJobToken(job.ID, auth.HashToken(token)); err != nil {
		return true, err
	}
	_ = d.store.SetBuildJobState(job.ID, store.BuildRunning, "")

	image := d.cfg.BuilderImages[job.Language]
	env := d.buildEnv(job.ID, token)

	runCtx := ctx
	if d.cfg.BuildTimeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, d.cfg.BuildTimeout)
		defer cancel()
	}

	if err := d.launcher.Run(runCtx, image, env); err != nil {
		// The container exited abnormally without reporting a result. Mark the
		// job/version failed if the builder did not already do so.
		d.failIfUnfinished(job.ID, v, collection, err)
		return true, err
	}
	return true, nil
}

func (d *Dispatcher) buildEnv(jobID, token string) map[string]string {
	env := map[string]string{
		"BUILD_JOB_ID":   jobID,
		"BUILD_TOKEN":    token,
		"REGISTRY_URL":   d.registryURL,
		"STAGING_PREFIX": d.cfg.StagingPrefix,
		"S3_ENDPOINT":    d.cfg.S3Endpoint,
		"S3_REGION":      d.cfg.S3Region,
		"S3_BUCKET":      d.cfg.S3Bucket,
	}
	if d.cfg.S3AccessKey != "" {
		env["S3_ACCESS_KEY_ID"] = d.cfg.S3AccessKey
		env["S3_SECRET_ACCESS_KEY"] = d.cfg.S3SecretKey
	}
	if d.cfg.S3UsePathStyle {
		env["S3_USE_PATH_STYLE"] = "true"
	}
	return env
}

// reapMargin is added to the build timeout before a claimed/running job counts
// as stuck, so the reaper never races a live build that the owning dispatcher
// is still going to time out and fail itself.
const reapMargin = 5 * time.Minute

// Reap sweeps for jobs stuck in claimed/running past the stale window — left
// behind by a dispatcher that died between claiming and recording a terminal
// state. Jobs with attempts remaining are requeued (the version returns to
// submitted for a clean re-dispatch); exhausted jobs are abandoned as failed.
func (d *Dispatcher) Reap() error {
	window := d.cfg.BuildTimeout
	if window <= 0 {
		window = 15 * time.Minute
	}
	stuck, err := d.store.ListStuckBuildJobs(time.Now().Add(-(window + reapMargin)))
	if err != nil {
		return err
	}
	var firstErr error
	for i := range stuck {
		if err := d.reapOne(&stuck[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *Dispatcher) reapOne(job *store.BuildJob) error {
	v, collection, err := d.jobContext(job)
	if err != nil {
		return err
	}
	switch {
	case v.State == store.StateAvailable:
		// The build finished; only the job row missed its terminal update.
		return d.store.SetBuildJobState(job.ID, store.BuildSucceeded, "")
	case v.State == store.StateScreened:
		// Held at the human-approval gate (REQUIRE_APPROVAL) — not stuck.
		return nil
	case job.Attempts >= d.MaxAttempts:
		reason := fmt.Sprintf("build abandoned after %d attempts", job.Attempts)
		d.log.Warn("abandoning stuck build job", "job", job.ID, "collection", collection,
			"version", v.Version, "attempts", job.Attempts)
		if state.CanTransition(v.State, store.StateFailed) {
			_ = d.state.Transition(systemActor, v, collection, store.StateFailed, reason, reason)
		}
		return d.store.SetBuildJobState(job.ID, store.BuildFailed, "")
	case v.State == store.StateSubmitted:
		// Died between claim and the screening transition; just requeue.
		d.log.Warn("requeueing stuck build job", "job", job.ID, "collection", collection,
			"version", v.Version, "attempts", job.Attempts)
		return d.store.RequeueBuildJob(job.ID)
	case state.CanTransition(v.State, store.StateSubmitted):
		d.log.Warn("requeueing stuck build job", "job", job.ID, "collection", collection,
			"version", v.Version, "attempts", job.Attempts)
		if err := d.state.Transition(systemActor, v, collection, store.StateSubmitted,
			"requeued: build runner did not complete", ""); err != nil {
			return err
		}
		return d.store.RequeueBuildJob(job.ID)
	default:
		// The version is in a state no rebuild can proceed from (failed,
		// recalled, ...); close out the orphaned job row.
		return d.store.SetBuildJobState(job.ID, store.BuildFailed, "")
	}
}

func (d *Dispatcher) failIfUnfinished(jobID string, v *store.Version, collection string, cause error) {
	fresh, err := d.store.GetBuildJob(jobID)
	if err != nil {
		return
	}
	if fresh.State == store.BuildSucceeded || fresh.State == store.BuildFailed {
		return // builder already reported a terminal result
	}
	cur, err := d.store.GetVersionByID(v.ID)
	if err == nil {
		_ = d.state.Transition(systemActor, cur, collection, store.StateFailed, "build container failed: "+cause.Error(), cause.Error())
	}
	_ = d.store.SetBuildJobState(jobID, store.BuildFailed, "")
}

func (d *Dispatcher) jobContext(job *store.BuildJob) (*store.Version, string, error) {
	v, err := d.store.GetVersionByID(job.VersionID)
	if err != nil {
		return nil, "", err
	}
	var col store.Collection
	if err := d.store.DB().Where("id = ?", v.CollectionID).First(&col).Error; err != nil {
		return nil, "", err
	}
	return v, col.Name, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
