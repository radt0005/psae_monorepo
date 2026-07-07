package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"spade_kms/internal/envelope"
	"spade_kms/internal/store"
)

// userID reads the authenticated user from the request. Better Auth
// integration is deferred (hosting.md §7); until then the header identifies the
// caller, matching server/api.
func userID(c echo.Context) (string, error) {
	id := c.Request().Header.Get("X-Spade-User-Id")
	if id == "" {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "missing X-Spade-User-Id")
	}
	return id, nil
}

type setRequest struct {
	Value string `json:"value"`
}

// handleSet stores (creates or updates) an envelope-encrypted secret.
func (s *Server) handleSet(c echo.Context) error {
	owner, err := userID(c)
	if err != nil {
		return err
	}
	name := c.Param("name")
	var req setRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Value == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "empty secret value")
	}

	sealed, err := s.keys.Seal([]byte(req.Value))
	if err != nil {
		s.logger.Error("sealing secret", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "encryption failed")
	}
	rec := store.Secret{
		OwnerID:    owner,
		Name:       name,
		Ciphertext: sealed.Ciphertext,
		ValueNonce: sealed.ValueNonce,
		WrappedDEK: sealed.WrappedDEK,
		DEKNonce:   sealed.DEKNonce,
		KEKID:      sealed.KEKID,
	}
	if err := s.store.Upsert(c.Request().Context(), rec); err != nil {
		s.logger.Error("storing secret", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "storage failed")
	}
	s.audit(c, store.AuditRecord{OwnerID: owner, SecretNames: name, Actor: owner, Action: "set"})
	return c.NoContent(http.StatusNoContent)
}

// handleList returns the caller's secret names (never values).
func (s *Server) handleList(c echo.Context) error {
	owner, err := userID(c)
	if err != nil {
		return err
	}
	names, err := s.store.ListNames(c.Request().Context(), owner)
	if err != nil {
		s.logger.Error("listing secrets", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "storage failed")
	}
	return c.JSON(http.StatusOK, map[string][]string{"names": names})
}

// handleDelete removes a secret.
func (s *Server) handleDelete(c echo.Context) error {
	owner, err := userID(c)
	if err != nil {
		return err
	}
	name := c.Param("name")
	if err := s.store.Delete(c.Request().Context(), owner, name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "secret not found")
		}
		s.logger.Error("deleting secret", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "storage failed")
	}
	s.audit(c, store.AuditRecord{OwnerID: owner, SecretNames: name, Actor: owner, Action: "delete"})
	return c.NoContent(http.StatusNoContent)
}

type resolveRequest struct {
	Token string   `json:"token"`
	Names []string `json:"names"`
}

// handleResolve exchanges a capability token for the scoped secret values. This
// is the only endpoint that returns plaintext (spec/secrets.md §5.3, §6.2).
func (s *Server) handleResolve(c echo.Context) error {
	var req resolveRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	claims, err := s.verifier.Verify(req.Token)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}
	if claims.Expired(time.Now()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "capability token expired")
	}

	out := make(map[string]string, len(req.Names))
	for _, name := range req.Names {
		// The requested names must be a subset of the token's scope.
		if !claims.Allows(name) {
			return echo.NewHTTPError(http.StatusForbidden, "secret not in token scope: "+name)
		}
		sec, err := s.store.Get(c.Request().Context(), claims.UserID, name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "secret not found: "+name)
			}
			s.logger.Error("loading secret", "err", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "storage failed")
		}
		val, err := s.keys.Open(envelope.Sealed{
			Ciphertext: sec.Ciphertext,
			ValueNonce: sec.ValueNonce,
			WrappedDEK: sec.WrappedDEK,
			DEKNonce:   sec.DEKNonce,
			KEKID:      sec.KEKID,
		})
		if err != nil {
			s.logger.Error("decrypting secret", "err", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "decryption failed")
		}
		out[name] = string(val)

		// Lazy KEK rotation (spec/secrets.md §9): if this secret is wrapped
		// under a non-active KEK, re-wrap it under the active one so the old
		// KEK can eventually be retired. Best-effort — never fail the resolve.
		if sec.KEKID != s.keys.ActiveID() {
			s.rewrap(c, claims.UserID, name, val)
		}
	}

	s.audit(c, store.AuditRecord{
		OwnerID:      claims.UserID,
		InvocationID: claims.InvocationID,
		SecretNames:  strings.Join(req.Names, ","),
		Actor:        "worker",
		Action:       "resolve",
	})
	return c.JSON(http.StatusOK, out)
}

// rewrap re-seals a secret under the active KEK and stores it. Best-effort: any
// failure is logged, not surfaced — the resolve already succeeded.
func (s *Server) rewrap(c echo.Context, owner, name string, value []byte) {
	sealed, err := s.keys.Seal(value)
	if err != nil {
		s.logger.Error("re-wrapping secret", "err", err, "name", name)
		return
	}
	if err := s.store.Upsert(c.Request().Context(), store.Secret{
		OwnerID:    owner,
		Name:       name,
		Ciphertext: sealed.Ciphertext,
		ValueNonce: sealed.ValueNonce,
		WrappedDEK: sealed.WrappedDEK,
		DEKNonce:   sealed.DEKNonce,
		KEKID:      sealed.KEKID,
	}); err != nil {
		s.logger.Error("storing re-wrapped secret", "err", err, "name", name)
	}
}

// audit writes an audit record, logging (but not failing the request) on error.
func (s *Server) audit(c echo.Context, rec store.AuditRecord) {
	if err := s.store.Audit(c.Request().Context(), rec); err != nil {
		s.logger.Error("writing audit record", "err", err, "action", rec.Action)
	}
}
