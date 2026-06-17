package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"spade_registry/internal/auth"
	"spade_registry/internal/store"
)

// context keys for values stashed by middleware.
const (
	ctxDeveloper = "dev"
	ctxWorker    = "worker"
	ctxBuildJob  = "buildjob"
)

// bearerToken extracts the token from an Authorization: Bearer header.
func bearerToken(c *echo.Context) string {
	h := c.Request().Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return after
	}
	return ""
}

// requireDeveloper validates a Better Auth session token.
func (s *Server) requireDeveloper(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		dev, err := s.dev.Verify(bearerToken(c))
		if err != nil {
			return errJSON(c, http.StatusUnauthorized, "invalid or missing developer session")
		}
		c.Set(ctxDeveloper, dev)
		return next(c)
	}
}

// requireWorker validates a rotated service token.
func (s *Server) requireWorker(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		w, err := s.worker.Verify(bearerToken(c))
		if err != nil {
			return errJSON(c, http.StatusUnauthorized, "invalid or missing worker service token")
		}
		c.Set(ctxWorker, w)
		return next(c)
	}
}

// requireBuilder validates the per-job builder token against the :id job.
func (s *Server) requireBuilder(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		job, err := s.builder.Verify(c.Param("id"), bearerToken(c))
		if err != nil {
			return errJSON(c, http.StatusUnauthorized, "invalid build token")
		}
		c.Set(ctxBuildJob, job)
		return next(c)
	}
}

// developer returns the authenticated developer set by requireDeveloper.
func developer(c *echo.Context) auth.Developer {
	d, _ := c.Get(ctxDeveloper).(auth.Developer)
	return d
}

// buildJob returns the build job set by requireBuilder.
func buildJob(c *echo.Context) *store.BuildJob {
	j, _ := c.Get(ctxBuildJob).(*store.BuildJob)
	return j
}
