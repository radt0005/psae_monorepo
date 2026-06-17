package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"spade_registry/internal/store"
	"spade_registry/internal/wire"
)

// listCollections handles GET /collections. Collections whose only versions are
// deprecated/recalled are still listed at the collection level; per-version
// browse filtering is the editor's concern via the metadata mirror.
func (s *Server) listCollections(c *echo.Context) error {
	cs, err := s.store.ListCollections()
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "listing collections")
	}
	out := make([]wire.CollectionInfo, 0, len(cs))
	for _, col := range cs {
		out = append(out, wire.CollectionInfo{
			Name:        col.Name,
			Language:    col.Language,
			Description: col.Description,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// listVersions handles GET /collections/:name/versions. The worker reads this to
// re-check a version's state during recall-freshness checks (worker.md §Recall).
func (s *Server) listVersions(c *echo.Context) error {
	vs, err := s.store.ListVersions(c.Param("name"))
	if errors.Is(err, store.ErrNotFound) {
		return errJSON(c, http.StatusNotFound, "collection not found")
	}
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "listing versions")
	}
	out := make([]wire.VersionInfo, 0, len(vs))
	for _, v := range vs {
		out = append(out, wire.VersionInfo{Version: v.Version, State: string(v.State)})
	}
	return c.JSON(http.StatusOK, out)
}

// pubkeys handles GET /pubkeys, returning the trusted public key set.
func (s *Server) pubkeys(c *echo.Context) error {
	keys, err := s.keyset.PublicKeys()
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "loading public keys")
	}
	return c.JSON(http.StatusOK, wire.PubKeysResponse{Keys: keys})
}
