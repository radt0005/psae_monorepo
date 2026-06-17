package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v5"

	"spade_registry/internal/audit"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
	"spade_registry/internal/wire"
)

// Build target (registry.md §5: Debian Linux on amd64).
const (
	targetPlatform = "linux"
	targetArch     = "amd64"
)

func (s *Server) artifactKey(collection, version string) string {
	return s.cfg.ArtifactPrefix + collection + "/" + version + "/" + targetPlatform + "/" + targetArch + ".tar.gz"
}
func (s *Server) stagingKey(collection, version string) string {
	return s.cfg.StagingPrefix + collection + "/" + version + "/" + targetPlatform + "/" + targetArch + ".tar.gz"
}

// loadJobContext returns the build job, its version, and collection name.
func (s *Server) loadJobContext(job *store.BuildJob) (*store.Version, string, error) {
	v, err := s.store.GetVersionByID(job.VersionID)
	if err != nil {
		return nil, "", err
	}
	var col store.Collection
	if err := s.store.DB().Where("id = ?", v.CollectionID).First(&col).Error; err != nil {
		return nil, "", err
	}
	return v, col.Name, nil
}

// getBuild handles GET /builds/:id.
func (s *Server) getBuild(c *echo.Context) error {
	job := buildJob(c)
	v, collection, err := s.loadJobContext(job)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading job context")
	}
	return c.JSON(http.StatusOK, wire.BuildJobDetail{
		JobID:         job.ID,
		VersionID:     v.ID,
		Collection:    collection,
		Version:       v.Version,
		RepoURL:       v.RepoURL,
		CommitSHA:     v.CommitSHA,
		Language:      job.Language,
		Platform:      targetPlatform,
		Arch:          targetArch,
		StagingPrefix: s.cfg.StagingPrefix,
		StagingKey:    s.stagingKey(collection, v.Version),
		ArtifactKey:   s.artifactKey(collection, v.Version),
	})
}

var systemActor = state.Actor{ID: "build-pipeline", Type: audit.ActorSystem}

// reportScreening handles POST /builds/:id/screening.
func (s *Server) reportScreening(c *echo.Context) error {
	job := buildJob(c)
	var req wire.ScreeningReport
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request body")
	}
	v, collection, err := s.loadJobContext(job)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading job context")
	}

	detail, _ := json.Marshal(map[string]string{"details": req.Details})
	_ = s.store.CreateScreeningResult(&store.ScreeningResult{
		VersionID:       v.ID,
		ScreenerName:    req.ScreenerName,
		ScreenerVersion: req.ScreenerVersion,
		Passed:          req.Passed,
		Details:         detail,
	})

	if !req.Passed {
		_ = s.state.Transition(systemActor, v, collection, store.StateFailed, "screening failed", req.Details)
		_ = s.store.SetBuildJobState(job.ID, store.BuildFailed, "")
		return c.JSON(http.StatusOK, wire.ScreeningAck{Proceed: false})
	}

	// screening → screened
	if err := s.state.Transition(systemActor, v, collection, store.StateScreened, "screening passed", ""); err != nil {
		return errJSON(c, http.StatusConflict, err.Error())
	}
	// Human-approval gate (registry.md §4): when required, do not auto-build.
	if s.cfg.RequireApproval {
		return c.JSON(http.StatusOK, wire.ScreeningAck{Proceed: false})
	}
	// screened → building
	if err := s.state.Transition(systemActor, v, collection, store.StateBuilding, "auto-build", ""); err != nil {
		return errJSON(c, http.StatusConflict, err.Error())
	}
	return c.JSON(http.StatusOK, wire.ScreeningAck{Proceed: true})
}

// completeBuild handles POST /builds/:id/complete: verify the staged artifact's
// hash, sign it with the active key (the builder never holds the key), store the
// signed artifact + signature, record metadata, and mark the version available.
func (s *Server) completeBuild(c *echo.Context) error {
	job := buildJob(c)
	var req wire.CompleteRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request body")
	}
	ctx := c.Request().Context()
	v, collection, err := s.loadJobContext(job)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading job context")
	}

	// Read the staged bytes once: verify hash and sign the same bytes.
	rc, err := s.blob.Get(ctx, req.StagingKey)
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "staged artifact not found")
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "reading staged artifact")
	}
	sum := sha256.Sum256(data)
	computed := hex.EncodeToString(sum[:])
	if computed != req.ContentHash {
		// Hash mismatch: do NOT sign or promote — the registry controls bytes.
		_ = s.state.Transition(systemActor, v, collection, store.StateFailed, "content hash mismatch", "content hash mismatch")
		_ = s.store.SetBuildJobState(job.ID, store.BuildFailed, "")
		return errJSON(c, http.StatusBadRequest, "content hash mismatch")
	}

	sig, keyID, err := s.keyset.SignArtifact(data)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "signing artifact")
	}

	artKey := s.artifactKey(collection, v.Version)
	sigKey := artKey + ".sig"
	if err := s.blob.Put(ctx, artKey, bytesReader(data), int64(len(data)), "application/gzip"); err != nil {
		return errJSON(c, http.StatusInternalServerError, "storing artifact")
	}
	if err := s.blob.Put(ctx, sigKey, bytesReader(sig), int64(len(sig)), "application/octet-stream"); err != nil {
		return errJSON(c, http.StatusInternalServerError, "storing signature")
	}

	if err := s.store.CreateArtifact(&store.Artifact{
		VersionID:    v.ID,
		Platform:     req.Platform,
		Arch:         req.Arch,
		ContentHash:  req.ContentHash,
		ArtifactKey:  artKey,
		SigKey:       sigKey,
		SigningKeyID: keyID,
		SizeBytes:    int64(len(data)),
	}); err != nil {
		return errJSON(c, http.StatusInternalServerError, "recording artifact")
	}

	// Persist block metadata for the mirror, then promote to available.
	_ = s.store.ReplaceBlockMeta(v.ID, blocksFromWire(req.Blocks))
	if err := s.state.Transition(systemActor, v, collection, store.StateAvailable, "build complete", ""); err != nil {
		return errJSON(c, http.StatusConflict, err.Error())
	}
	_ = s.store.SetBuildJobState(job.ID, store.BuildSucceeded, "")

	// Best-effort staging cleanup.
	_ = s.blob.Delete(ctx, req.StagingKey)
	return c.JSON(http.StatusOK, wire.VersionInfo{Version: v.Version, State: string(store.StateAvailable)})
}

// failBuild handles POST /builds/:id/fail.
func (s *Server) failBuild(c *echo.Context) error {
	job := buildJob(c)
	var req wire.FailRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request body")
	}
	v, collection, err := s.loadJobContext(job)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading job context")
	}
	if req.LogsKey != "" {
		_ = s.store.SetBuildJobLogs(job.ID, req.LogsKey)
	}
	// A failed build can come from any pre-available state; only transition if
	// the machine permits it (e.g. building/screening → failed).
	if err := s.state.Transition(systemActor, v, collection, store.StateFailed, req.Reason, req.Reason); err != nil &&
		!errors.Is(err, state.ErrIllegalTransition) {
		return errJSON(c, http.StatusInternalServerError, "marking failed")
	}
	_ = s.store.SetBuildJobState(job.ID, store.BuildFailed, "")
	return c.JSON(http.StatusOK, wire.VersionInfo{Version: v.Version, State: string(v.State)})
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func blocksFromWire(in []wire.BlockManifestWire) []store.BlockMeta {
	out := make([]store.BlockMeta, 0, len(in))
	for _, b := range in {
		inputs, _ := json.Marshal(b.Inputs)
		outputs, _ := json.Marshal(b.Outputs)
		out = append(out, store.BlockMeta{
			BlockID:     b.ID,
			Name:        b.Name,
			Kind:        b.Kind,
			Network:     b.Network,
			Description: b.Description,
			Entrypoint:  b.Entrypoint,
			Inputs:      inputs,
			Outputs:     outputs,
		})
	}
	return out
}
