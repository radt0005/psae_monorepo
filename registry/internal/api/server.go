// Package api implements the registry control-plane HTTP/JSON API (registry.md
// §11): developer publish + status + state, public/worker browse + fetch +
// pubkeys, and the internal builder-facing endpoints. It is built on Echo v5.
package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"spade_registry/internal/audit"
	"spade_registry/internal/auth"
	"spade_registry/internal/blob"
	"spade_registry/internal/config"
	"spade_registry/internal/sign"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
	"spade_registry/internal/wire"
)

// Server holds the API's collaborators.
type Server struct {
	cfg    config.RegistryConfig
	store  *store.Store
	keyset *sign.Keyset
	audit  *audit.Logger
	state  *state.Machine
	blob   blob.Store

	dev     auth.DeveloperVerifier
	worker  *auth.WorkerAuth
	builder *auth.BuilderAuth

	baseURL string // public registry URL, for status links
}

// Options configures a Server.
type Options struct {
	Config  config.RegistryConfig
	Store   *store.Store
	Keyset  *sign.Keyset
	Audit   *audit.Logger
	State   *state.Machine
	Blob    blob.Store
	Dev     auth.DeveloperVerifier
	BaseURL string
}

// New builds a Server.
func New(o Options) *Server {
	base := o.BaseURL
	if base == "" {
		base = "http://localhost" + o.Config.ListenAddr
	}
	return &Server{
		cfg:     o.Config,
		store:   o.Store,
		keyset:  o.Keyset,
		audit:   o.Audit,
		state:   o.State,
		blob:    o.Blob,
		dev:     o.Dev,
		worker:  auth.NewWorkerAuth(o.Store),
		builder: auth.NewBuilderAuth(o.Store),
		baseURL: base,
	}
}

// Routes builds the Echo router with all endpoints wired.
func (s *Server) Routes() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Recover())

	e.GET("/healthz", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Public / worker read endpoints.
	e.GET("/collections", s.listCollections)
	e.GET("/collections/:name/versions", s.listVersions)
	e.GET("/pubkeys", s.pubkeys)

	// Worker fetch (service token). The final segment is "<arch>",
	// "<arch>.tar.gz", or "<arch>.sig"; the handler routes on the suffix.
	e.GET("/artifacts/:name/:version/:platform/:artifact", s.fetchArtifact, s.requireWorker)

	// Developer endpoints (session token).
	e.POST("/publish", s.publish, s.requireDeveloper)
	e.GET("/collections/:name/:version/status", s.versionStatus, s.requireDeveloper)
	e.POST("/collections/:name/:version/state", s.changeState, s.requireDeveloper)

	// Builder-facing internal endpoints (per-job token).
	e.GET("/builds/:id", s.getBuild, s.requireBuilder)
	e.POST("/builds/:id/screening", s.reportScreening, s.requireBuilder)
	e.POST("/builds/:id/complete", s.completeBuild, s.requireBuilder)
	e.POST("/builds/:id/fail", s.failBuild, s.requireBuilder)

	return e
}

func errJSON(c *echo.Context, code int, msg string) error {
	return c.JSON(code, wire.ErrorResponse{Error: msg})
}
