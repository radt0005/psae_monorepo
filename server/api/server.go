// Package api exposes the spade-scheduler's HTTP surface.
//
// The web UI calls these endpoints to submit pipelines, browse status,
// and cancel runs (web_ui.md).  The endpoints are intentionally small —
// the scheduler is the coordination layer, not the application data
// layer.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"core"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gopkg.in/yaml.v3"

	"spade_server/engine"
	"spade_server/store"
)

// Server wires the Engine and Store to the HTTP router.
type Server struct {
	engine *engine.Engine
	store  store.Store
	logger *slog.Logger

	echo *echo.Echo
}

// New constructs a Server with the given dependencies.  Routes are
// registered eagerly; call Echo() to retrieve the underlying router for
// tests, or Start() to serve on a TCP address.
func New(e *engine.Engine, s store.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	ec := echo.New()
	ec.HideBanner = true
	ec.HidePort = true
	ec.Use(middleware.Recover())
	srv := &Server{engine: e, store: s, logger: logger, echo: ec}
	srv.routes()
	return srv
}

// Echo returns the underlying *echo.Echo, useful for httptest-based tests.
func (s *Server) Echo() *echo.Echo { return s.echo }

// Start binds to addr and serves.  Blocks until the server returns an
// error or Shutdown is called.
func (s *Server) Start(addr string) error {
	if err := s.echo.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error { return s.echo.Shutdown(ctx) }

func (s *Server) routes() {
	s.echo.GET("/", s.handleHealth)
	s.echo.GET("/healthz", s.handleHealth)
	s.echo.POST("/pipelines", s.handleSubmitPipeline)
	s.echo.GET("/pipelines", s.handleListPipelines)
	s.echo.GET("/pipelines/:id", s.handleGetPipeline)
	s.echo.DELETE("/pipelines/:id", s.handleCancelPipeline)
	s.echo.GET("/pipelines/:id/invocations", s.handleListInvocations)
	s.echo.GET("/invocations/:id", s.handleGetInvocation)
	s.echo.GET("/invocations/:id/logs", s.handleGetInvocationLogs)
}

func (s *Server) handleHealth(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

// submitPipelineRequest is the body of POST /pipelines.  Callers may
// supply either YAML in the raw body (Content-Type: application/yaml or
// text/yaml) or a JSON object whose "yaml" field contains the YAML
// pipeline source.  The structural Pipeline form is also accepted.
type submitPipelineRequest struct {
	YAML     string        `json:"yaml,omitempty"`
	Pipeline *core.Pipeline `json:"pipeline,omitempty"`
}

// submitPipelineResponse is the success body of POST /pipelines.
type submitPipelineResponse struct {
	ID      uuid.UUID            `json:"id"`
	Name    string               `json:"name"`
	Version string               `json:"version"`
	Status  store.PipelineStatus `json:"status"`
}

func (s *Server) handleSubmitPipeline(c echo.Context) error {
	ctx := c.Request().Context()
	ct := c.Request().Header.Get(echo.HeaderContentType)
	var p core.Pipeline
	var rawYAML []byte

	switch {
	case ct == "application/yaml" || ct == "text/yaml":
		body, err := readAll(c.Request().Body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "reading body: " + err.Error()})
		}
		if err := yaml.Unmarshal(body, &p); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "parsing yaml: " + err.Error()})
		}
		rawYAML = body
	default:
		var req submitPipelineRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if req.Pipeline != nil {
			p = *req.Pipeline
		} else if req.YAML != "" {
			rawYAML = []byte(req.YAML)
			if err := yaml.Unmarshal(rawYAML, &p); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "parsing yaml: " + err.Error()})
			}
		} else {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing pipeline body"})
		}
	}

	submitter := c.Request().Header.Get("X-Spade-User-Id")
	if err := s.engine.SubmitPipeline(ctx, &p, rawYAML, submitter); err != nil {
		var ve *engine.ValidationError
		if errors.As(err, &ve) {
			errs := make([]string, 0, len(ve.Errors))
			for _, e := range ve.Errors {
				errs = append(errs, e.Error())
			}
			return c.JSON(http.StatusBadRequest, map[string]any{"validation_errors": errs})
		}
		if errors.Is(err, store.ErrAlreadyExists) {
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, submitPipelineResponse{
		ID:      p.Id,
		Name:    p.Name,
		Version: p.Version,
		Status:  store.PipelineRunning,
	})
}

// pipelineSummaryJSON is the JSON view of store.PipelineSummary.
type pipelineSummaryJSON struct {
	ID          uuid.UUID            `json:"id"`
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Status      store.PipelineStatus `json:"status"`
	SubmittedAt time.Time            `json:"submitted_at"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
}

func (s *Server) handleListPipelines(c echo.Context) error {
	q := c.QueryParam("status")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	filter := store.ListFilter{
		Status:    store.PipelineStatus(q),
		Submitter: c.QueryParam("submitter"),
		Limit:     limit,
		Offset:    offset,
	}
	sums, err := s.store.ListPipelines(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	out := make([]pipelineSummaryJSON, 0, len(sums))
	for _, sum := range sums {
		out = append(out, pipelineSummaryJSON{
			ID: sum.ID, Name: sum.Name, Version: sum.Version,
			Status: sum.Status, SubmittedAt: sum.SubmittedAt, CompletedAt: sum.CompletedAt,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// pipelineStatusJSON is the JSON form returned by GET /pipelines/:id.
type pipelineStatusJSON struct {
	ID          uuid.UUID            `json:"id"`
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Status      store.PipelineStatus `json:"status"`
	SubmittedAt time.Time            `json:"submitted_at"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
	Cancelled   bool                 `json:"cancelled"`
	Complete    bool                 `json:"complete"`
	Failed      bool                 `json:"failed"`
	Blocks      []blockStatusJSON    `json:"blocks"`
}

// blockStatusJSON is the JSON form for one block's snapshot.
type blockStatusJSON struct {
	BlockID        uuid.UUID                `json:"block_id"`
	Name           string                   `json:"name"`
	Status         core.BlockSnapshotStatus `json:"status"`
	MapIndex       *int                     `json:"map_index,omitempty"`
	MapInvocations []string                 `json:"map_invocations,omitempty"`
	ExitCode       int                      `json:"exit_code"`
	ErrorMessage   string                   `json:"error,omitempty"`
}

func (s *Server) handleGetPipeline(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	view, err := s.engine.PipelineStatus(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	resp := pipelineStatusJSON{
		ID: view.ID, Name: view.Name, Version: view.Version,
		Status: view.Status, SubmittedAt: view.SubmittedAt, CompletedAt: view.CompletedAt,
		Cancelled: view.Cancelled, Complete: view.Complete, Failed: view.Failed,
	}
	for _, b := range view.Blocks {
		resp.Blocks = append(resp.Blocks, blockStatusJSON{
			BlockID: b.BlockID, Name: b.Name, Status: b.Status,
			MapIndex: b.MapIndex, MapInvocations: b.MapInvocations,
			ExitCode: b.ExitCode, ErrorMessage: b.ErrorMessage,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleCancelPipeline(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if err := s.engine.CancelPipeline(c.Request().Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.NoContent(http.StatusNoContent)
}

// invocationJSON is the JSON view of store.InvocationRecord.
type invocationJSON struct {
	ID           string                 `json:"id"`
	PipelineID   uuid.UUID              `json:"pipeline_id"`
	BlockID      uuid.UUID              `json:"block_id"`
	BlockName    string                 `json:"block_name"`
	MapIndex     *int                   `json:"map_index,omitempty"`
	Status       store.InvocationStatus `json:"status"`
	DispatchedAt *time.Time             `json:"dispatched_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	ExitCode     int                    `json:"exit_code"`
	LogsPath     string                 `json:"logs_path,omitempty"`
	ErrorMessage string                 `json:"error,omitempty"`
}

func (s *Server) handleListInvocations(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	rows, err := s.store.LoadInvocations(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	out := make([]invocationJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, invocationToJSON(r))
	}
	return c.JSON(http.StatusOK, out)
}

func (s *Server) handleGetInvocation(c echo.Context) error {
	rec, err := s.store.LoadInvocation(c.Request().Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, invocationToJSON(rec))
}

func (s *Server) handleGetInvocationLogs(c echo.Context) error {
	rec, err := s.store.LoadInvocation(c.Request().Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if rec.LogsPath == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no logs available yet"})
	}
	return c.JSON(http.StatusOK, map[string]string{"logs_path": rec.LogsPath})
}

func invocationToJSON(r store.InvocationRecord) invocationJSON {
	return invocationJSON{
		ID: r.ID, PipelineID: r.PipelineID, BlockID: r.BlockID,
		BlockName: r.BlockName, MapIndex: r.MapIndex, Status: r.Status,
		DispatchedAt: r.DispatchedAt, CompletedAt: r.CompletedAt,
		ExitCode: r.ExitCode, LogsPath: r.LogsPath, ErrorMessage: r.ErrorMessage,
	}
}
