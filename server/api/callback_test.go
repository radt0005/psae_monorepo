package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"spade_server/engine"
	"spade_server/store"
)

func TestPatchRunStatus_Success(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody callbackBody

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	payload := engine.CallbackPayload{Status: store.PipelineComplete}
	if err := PatchRunStatus(context.Background(), srv.URL, "my-secret", "run-123", payload); err != nil {
		t.Fatalf("PatchRunStatus: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Errorf("method: got %s, want PATCH", gotMethod)
	}
	if gotPath != "/api/runs/run-123" {
		t.Errorf("path: got %s, want /api/runs/run-123", gotPath)
	}
	if gotAuth != "Bearer my-secret" {
		t.Errorf("auth header: got %q, want 'Bearer my-secret'", gotAuth)
	}
	if gotBody.Status != "succeeded" {
		t.Errorf("status in body: got %q, want 'succeeded'", gotBody.Status)
	}
}

func TestPatchRunStatus_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	payload := engine.CallbackPayload{Status: store.PipelineFailed}
	err := PatchRunStatus(context.Background(), srv.URL, "secret", "run-abc", payload)
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

func TestPatchRunStatus_EmptyBaseURL(t *testing.T) {
	// Should be a no-op and return nil.
	payload := engine.CallbackPayload{Status: store.PipelineComplete}
	if err := PatchRunStatus(context.Background(), "", "secret", "run-xyz", payload); err != nil {
		t.Fatalf("expected nil for empty baseURL, got %v", err)
	}
}

func TestPatchRunStatus_EmptySecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when secret is empty")
	}))
	defer srv.Close()

	payload := engine.CallbackPayload{Status: store.PipelineComplete}
	if err := PatchRunStatus(context.Background(), srv.URL, "", "run-xyz", payload); err != nil {
		t.Fatalf("expected nil for empty secret, got %v", err)
	}
}

func TestTranslateStatus(t *testing.T) {
	cases := []struct {
		in   store.PipelineStatus
		want string
	}{
		{store.PipelineRunning, "running"},
		{store.PipelineComplete, "succeeded"},
		{store.PipelineFailed, "failed"},
		{store.PipelineCancelled, "canceled"}, // one 'l' in web UI enum
	}
	for _, tc := range cases {
		got := TranslateStatus(tc.in)
		if got != tc.want {
			t.Errorf("TranslateStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildCallbackBody_FilesAndLogs(t *testing.T) {
	payload := engine.CallbackPayload{
		Status:   store.PipelineComplete,
		ErrorMsg: "",
		Invocations: []store.InvocationRecord{
			{
				ID:               "inv-001",
				OutputHashesJSON: `{"result":"abc123"}`,
				LogsPath:         "s3://bucket/logs/inv-001/stdout",
			},
			{
				ID:               "inv-002",
				OutputHashesJSON: "", // no outputs
				LogsPath:         "",
			},
		},
	}

	body := buildCallbackBody(payload)

	if body.Status != "succeeded" {
		t.Errorf("status: got %q, want 'succeeded'", body.Status)
	}
	if len(body.Files) != 1 {
		t.Fatalf("files: got %d, want 1", len(body.Files))
	}
	if body.Files[0].Name != "result" {
		t.Errorf("file name: got %q, want 'result'", body.Files[0].Name)
	}
	if body.Files[0].S3Key != "outputs/inv-001/result" {
		t.Errorf("file s3Key: got %q, want 'outputs/inv-001/result'", body.Files[0].S3Key)
	}
	if body.Files[0].BlockID != "inv-001" {
		t.Errorf("file blockId: got %q, want 'inv-001'", body.Files[0].BlockID)
	}
	if len(body.Logs) != 1 {
		t.Fatalf("logs: got %d, want 1", len(body.Logs))
	}
	if body.Logs[0].Stdout != "s3://bucket/logs/inv-001/stdout" {
		t.Errorf("log stdout: got %q", body.Logs[0].Stdout)
	}
}
