package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"spade_registry/internal/audit"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
	"spade_registry/internal/wire"
)

// publish handles POST /publish: record a publish request and enqueue a build.
func (s *Server) publish(c *echo.Context) error {
	var req wire.PublishRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request body")
	}
	req.Collection = strings.TrimSpace(req.Collection)
	req.Version = strings.TrimSpace(req.Version)
	if req.Collection == "" || req.Version == "" || req.RepoURL == "" || req.CommitSHA == "" {
		return errJSON(c, http.StatusBadRequest, "repo_url, commit_sha, collection, version are required")
	}
	dev := developer(c)

	col, created, err := s.store.EnsureCollection(req.Collection, dev.UserID, req.Language)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "ensuring collection")
	}
	if !created && col.OwnerUserID != dev.UserID && !s.cfg.IsAdmin(dev.UserID) {
		return errJSON(c, http.StatusForbidden, "you do not own this collection")
	}

	// Reject a duplicate version unless the prior attempt failed (re-publish).
	existing, err := s.store.GetVersion(req.Collection, req.Version)
	switch {
	case err == nil && existing.State != store.StateFailed:
		return errJSON(c, http.StatusConflict, "version already exists")
	case err == nil && existing.State == store.StateFailed:
		if e := s.store.SetVersionState(existing.ID, store.StateSubmitted, ""); e != nil {
			return errJSON(c, http.StatusInternalServerError, "resetting failed version")
		}
		if e := s.enqueueBuild(existing.ID, col.Language); e != nil {
			return errJSON(c, http.StatusInternalServerError, "enqueuing build")
		}
		_ = s.audit.Publish(dev.UserID, req.Collection, req.Version, req.RepoURL, req.CommitSHA)
		return c.JSON(http.StatusAccepted, s.publishResponse(existing.ID, req))
	case !errors.Is(err, store.ErrNotFound):
		return errJSON(c, http.StatusInternalServerError, "checking version")
	}

	v := &store.Version{
		ID:                store.NewID(),
		CollectionID:      col.ID,
		Version:           req.Version,
		State:             store.StateSubmitted,
		RepoURL:           req.RepoURL,
		CommitSHA:         req.CommitSHA,
		SubmittedByUserID: dev.UserID,
	}
	if err := s.store.CreateVersion(v); err != nil {
		return errJSON(c, http.StatusInternalServerError, "creating version")
	}
	if err := s.enqueueBuild(v.ID, col.Language); err != nil {
		return errJSON(c, http.StatusInternalServerError, "enqueuing build")
	}
	_ = s.audit.Publish(dev.UserID, req.Collection, req.Version, req.RepoURL, req.CommitSHA)
	return c.JSON(http.StatusAccepted, s.publishResponse(v.ID, req))
}

func (s *Server) enqueueBuild(versionID, language string) error {
	return s.store.CreateBuildJob(&store.BuildJob{
		ID:        store.NewID(),
		VersionID: versionID,
		Language:  language,
		State:     store.BuildQueued,
	})
}

func (s *Server) publishResponse(versionID string, req wire.PublishRequest) wire.PublishResponse {
	return wire.PublishResponse{
		VersionID: versionID,
		StatusURL: s.baseURL + "/collections/" + req.Collection + "/" + req.Version + "/status",
		State:     string(store.StateSubmitted),
	}
}

// versionStatus handles GET /collections/:name/:version/status.
func (s *Server) versionStatus(c *echo.Context) error {
	name, version := c.Param("name"), c.Param("version")
	col, err := s.store.GetCollectionByName(name)
	if errors.Is(err, store.ErrNotFound) {
		return errJSON(c, http.StatusNotFound, "collection not found")
	}
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading collection")
	}
	dev := developer(c)
	if col.OwnerUserID != dev.UserID && !s.cfg.IsAdmin(dev.UserID) {
		return errJSON(c, http.StatusForbidden, "not authorized to view this collection's status")
	}
	v, err := s.store.GetVersion(name, version)
	if errors.Is(err, store.ErrNotFound) {
		return errJSON(c, http.StatusNotFound, "version not found")
	}
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading version")
	}

	out := wire.VersionStatus{
		Collection: name,
		Version:    version,
		State:      string(v.State),
		Error:      v.Error,
	}
	if results, err := s.store.ListScreeningResults(v.ID); err == nil {
		for _, r := range results {
			out.Screening = append(out.Screening, wire.ScreeningInfo{
				ScreenerName:    r.ScreenerName,
				ScreenerVersion: r.ScreenerVersion,
				Passed:          r.Passed,
				Details:         string(r.Details),
			})
		}
	}
	if v.Artifacts == nil {
		var arts []store.Artifact
		s.store.DB().Where("version_id = ?", v.ID).Find(&arts)
		v.Artifacts = arts
	}
	for _, a := range v.Artifacts {
		out.Artifacts = append(out.Artifacts, wire.ArtifactInfo{
			Platform: a.Platform, Arch: a.Arch, ContentHash: a.ContentHash,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// changeState handles POST /collections/:name/:version/state (deprecate, yank,
// recall). Owner may deprecate/yank; operator may recall.
func (s *Server) changeState(c *echo.Context) error {
	name, version := c.Param("name"), c.Param("version")
	var req wire.StateChangeRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request body")
	}
	to := store.State(req.ToState)
	if !to.Valid() {
		return errJSON(c, http.StatusBadRequest, "invalid target state")
	}
	col, err := s.store.GetCollectionByName(name)
	if errors.Is(err, store.ErrNotFound) {
		return errJSON(c, http.StatusNotFound, "collection not found")
	}
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading collection")
	}
	v, err := s.store.GetVersion(name, version)
	if errors.Is(err, store.ErrNotFound) {
		return errJSON(c, http.StatusNotFound, "version not found")
	}
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading version")
	}

	dev := developer(c)
	isOperator := s.cfg.IsAdmin(dev.UserID)
	actor := state.Actor{
		ID:         dev.UserID,
		Type:       actorType(isOperator),
		IsOwner:    col.OwnerUserID == dev.UserID,
		IsOperator: isOperator,
	}
	if err := s.state.Transition(actor, v, name, to, req.Reason, ""); err != nil {
		switch {
		case errors.Is(err, state.ErrUnauthorized):
			return errJSON(c, http.StatusForbidden, err.Error())
		case errors.Is(err, state.ErrIllegalTransition):
			return errJSON(c, http.StatusConflict, err.Error())
		default:
			return errJSON(c, http.StatusInternalServerError, "applying transition")
		}
	}
	return c.JSON(http.StatusOK, wire.VersionInfo{Version: version, State: string(v.State)})
}

func actorType(isOperator bool) string {
	if isOperator {
		return audit.ActorOperator
	}
	return audit.ActorDeveloper
}
