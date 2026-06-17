package api

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v5"

	"spade_registry/internal/auth"
	"spade_registry/internal/store"
)

// fetchArtifact handles GET /artifacts/:name/:version/:platform/:artifact for
// workers. The final segment selects the tarball ("<arch>" or "<arch>.tar.gz")
// or the signature ("<arch>.sig"). State gating implements registry.md §3.1:
// available/deprecated serve; yanked/recalled (and any non-available) refuse so
// the worker invalidates its local index.
func (s *Server) fetchArtifact(c *echo.Context) error {
	name := c.Param("name")
	version := c.Param("version")
	platform := c.Param("platform")
	seg := c.Param("artifact")

	isSig := strings.HasSuffix(seg, ".sig")
	arch := strings.TrimSuffix(seg, ".sig")
	arch = strings.TrimSuffix(arch, ".tar.gz")

	art, v, err := s.store.GetArtifact(name, version, platform, arch)
	if errors.Is(err, store.ErrNotFound) {
		return errJSON(c, http.StatusNotFound, "artifact not found")
	}
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading artifact")
	}

	switch v.State {
	case store.StateAvailable, store.StateDeprecated:
		// serve
	case store.StateRecalled:
		return errJSON(c, http.StatusGone, "collection version recalled")
	case store.StateYanked:
		return errJSON(c, http.StatusGone, "collection version yanked: no new installs")
	default:
		return errJSON(c, http.StatusConflict, "collection version not available")
	}

	key, contentType := art.ArtifactKey, "application/gzip"
	if isSig {
		key, contentType = art.SigKey, "application/octet-stream"
	}
	rc, err := s.blob.Get(c.Request().Context(), key)
	if errors.Is(err, os.ErrNotExist) {
		return errJSON(c, http.StatusNotFound, "artifact bytes missing")
	}
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "opening artifact")
	}
	defer rc.Close()

	// Record the fetch for incident response (registry.md §7.3).
	if w, ok := c.Get(ctxWorker).(auth.Worker); ok {
		_ = s.audit.Fetch(w.ID, name, version, platform, arch)
	}

	c.Response().Header().Set(echo.HeaderContentType, contentType)
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), rc)
	return err
}
