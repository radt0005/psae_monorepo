package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"core"

	"github.com/google/uuid"

	"spade_server/broker"
	"spade_server/engine"
	"spade_server/store"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestAPI returns an api.Server backed by in-memory store + fake
// broker + a manifest provider pre-seeded for the linear test pipeline.
func newTestAPI(t *testing.T) (*Server, *store.MemStore, *engine.Engine) {
	t.Helper()
	mem := store.NewMemStore()
	pub := &broker.FakeJobPublisher{}
	mp := engine.NewMapManifestProvider()
	mp.Set("src", core.BlockManifest{
		ID: "test.src", Outputs: map[string]core.OutputDeclaration{"out": {Type: "file"}},
	})
	mp.Set("mid", core.BlockManifest{
		ID:      "test.mid",
		Inputs:  map[string]core.InputDeclaration{"in": {Type: "file"}},
		Outputs: map[string]core.OutputDeclaration{"out": {Type: "file"}},
	})
	eng := engine.New(mem, pub, mp, silentLogger())
	return New(eng, mem, silentLogger()), mem, eng
}

func TestHealthEndpoint(t *testing.T) {
	srv, _, _ := newTestAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("body: %q", rec.Body.String())
	}
}

func TestSubmitPipelineYAML(t *testing.T) {
	srv, mem, _ := newTestAPI(t)
	a := uuid.Must(uuid.NewV7())
	b := uuid.Must(uuid.NewV7())
	body := `name: tp
version: "1.0"
blocks:
  - id: ` + a.String() + `
    name: src
    inputs: []
    args: {}
  - id: ` + b.String() + `
    name: mid
    inputs:
      - ` + a.String() + `
    args: {}
`
	req := httptest.NewRequest(http.MethodPost, "/pipelines", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/yaml")
	req.Header.Set("X-Spade-User-Id", "alice")
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp submitPipelineResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != store.PipelineRunning {
		t.Errorf("status: %s", resp.Status)
	}
	stored, err := mem.LoadPipeline(context.Background(), resp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SubmitterUserID != "alice" {
		t.Errorf("submitter not persisted: %q", stored.SubmitterUserID)
	}
}

func TestSubmitPipelineValidationError(t *testing.T) {
	srv, _, _ := newTestAPI(t)
	// Reference a block type that isn't in the manifest provider.
	body := `{"yaml":"name: p\nversion: \"1\"\nblocks:\n  - id: ` +
		uuid.Must(uuid.NewV7()).String() + `\n    name: unknown\n    inputs: []\n    args: {}\n"}`
	req := httptest.NewRequest(http.MethodPost, "/pipelines", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetPipelineNotFound(t *testing.T) {
	srv, _, _ := newTestAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/pipelines/"+uuid.Must(uuid.NewV7()).String(), nil)
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListAndGetPipeline(t *testing.T) {
	srv, _, _ := newTestAPI(t)
	// Submit one pipeline.
	a := uuid.Must(uuid.NewV7())
	body := `name: tp
version: "1"
blocks:
  - id: ` + a.String() + `
    name: src
    inputs: []
    args: {}
`
	req := httptest.NewRequest(http.MethodPost, "/pipelines", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/yaml")
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit failed: %d %s", rec.Code, rec.Body.String())
	}
	var sub submitPipelineResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sub)

	// List.
	rec2 := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/pipelines", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("list: %d", rec2.Code)
	}
	var sums []pipelineSummaryJSON
	_ = json.Unmarshal(rec2.Body.Bytes(), &sums)
	if len(sums) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(sums))
	}

	// Detail.
	rec3 := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/pipelines/"+sub.ID.String(), nil))
	if rec3.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec3.Code, rec3.Body.String())
	}
	var view pipelineStatusJSON
	_ = json.Unmarshal(rec3.Body.Bytes(), &view)
	if len(view.Blocks) != 1 || view.Blocks[0].Name != "src" {
		t.Errorf("unexpected snapshot: %+v", view)
	}
}

func TestCancelPipeline(t *testing.T) {
	srv, mem, _ := newTestAPI(t)
	a := uuid.Must(uuid.NewV7())
	body := `name: tp
version: "1"
blocks:
  - id: ` + a.String() + `
    name: src
    inputs: []
    args: {}
`
	req := httptest.NewRequest(http.MethodPost, "/pipelines", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/yaml")
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)
	var sub submitPipelineResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sub)

	rec2 := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec2, httptest.NewRequest(http.MethodDelete, "/pipelines/"+sub.ID.String(), nil))
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("cancel: %d", rec2.Code)
	}
	rec3, _ := mem.LoadPipeline(context.Background(), sub.ID)
	if rec3.Status != store.PipelineCancelled {
		t.Errorf("status: %s", rec3.Status)
	}
}
