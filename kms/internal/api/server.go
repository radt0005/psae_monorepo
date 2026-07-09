// Package api exposes the KMS HTTP surface (spec/secrets.md §5.3): secret
// management for authenticated users and capability-token-gated resolution for
// the worker. Only /resolve returns plaintext values.
package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"spade_kms/internal/envelope"
	"spade_kms/internal/store"
	"spade_kms/internal/token"
)

// Server wires the store, key set, and token verifier to the HTTP router.
type Server struct {
	store    store.Store
	keys     *envelope.KeySet
	verifier token.Verifier
	logger   *slog.Logger
	echo     *echo.Echo
}

// New constructs a Server and registers routes.
func New(st store.Store, keys *envelope.KeySet, v token.Verifier, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	ec := echo.New()
	ec.HideBanner = true
	ec.HidePort = true
	ec.Use(middleware.Recover())
	s := &Server{store: st, keys: keys, verifier: v, logger: logger, echo: ec}
	s.routes()
	return s
}

// Echo returns the underlying router, useful for httptest-based tests.
func (s *Server) Echo() *echo.Echo { return s.echo }

// Start binds to addr and serves until Shutdown or an error.
func (s *Server) Start(addr string) error {
	if err := s.echo.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) routes() {
	s.echo.PUT("/secrets/:name", s.handleSet)
	s.echo.GET("/secrets", s.handleList)
	s.echo.DELETE("/secrets/:name", s.handleDelete)
	s.echo.POST("/resolve", s.handleResolve)
}
