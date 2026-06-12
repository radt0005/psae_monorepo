package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"spade_server/engine"
	"spade_server/store"
)

// callbackBody is the JSON body sent to PATCH /api/runs/:id.
// It matches the shape accepted by the Nuxt handler at
// web_ui/server/api/runs/[id]/index.patch.ts.
type callbackBody struct {
	Status string         `json:"status,omitempty"`
	Error  string         `json:"error,omitempty"`
	Files  []callbackFile `json:"files,omitempty"`
	Logs   []callbackLog  `json:"logs,omitempty"`
}

type callbackFile struct {
	Name    string `json:"name"`
	S3Key   string `json:"s3Key"`
	BlockID string `json:"blockId,omitempty"`
}

type callbackLog struct {
	BlockID string `json:"blockId,omitempty"`
	Stdout  string `json:"stdout,omitempty"`
}

// PatchRunStatus calls PATCH <baseURL>/api/runs/<runID> with a JSON body built
// from payload.  Returns nil on HTTP 200.  If baseURL or secret is empty the
// call is skipped and nil is returned.
func PatchRunStatus(ctx context.Context, baseURL, secret, runID string, payload engine.CallbackPayload) error {
	if baseURL == "" || secret == "" {
		return nil
	}

	body := buildCallbackBody(payload)
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling callback body: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url := baseURL + "/api/runs/" + runID
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending callback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("callback returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// TranslateStatus maps a Go PipelineStatus to the web UI's RunStatus enum.
// Note the spelling difference: Go uses "cancelled" (two l's); the web UI
// DB enum uses "canceled" (one l).
func TranslateStatus(s store.PipelineStatus) string {
	switch s {
	case store.PipelineRunning:
		return "running"
	case store.PipelineComplete:
		return "succeeded"
	case store.PipelineFailed:
		return "failed"
	case store.PipelineCancelled:
		return "canceled"
	default:
		return string(s)
	}
}

// buildCallbackBody converts a CallbackPayload to the JSON shape the web UI
// PATCH endpoint expects.  Files are built from OutputHashesJSON entries using
// the S3 key convention outputs/<invocation_id>/<output_name>.  Logs carry the
// S3 path from LogsPath as a Stdout hint.
func buildCallbackBody(payload engine.CallbackPayload) callbackBody {
	body := callbackBody{
		Status: TranslateStatus(payload.Status),
		Error:  payload.ErrorMsg,
	}
	for _, inv := range payload.Invocations {
		if inv.OutputHashesJSON != "" {
			var hashes map[string]string
			if err := json.Unmarshal([]byte(inv.OutputHashesJSON), &hashes); err == nil {
				for name := range hashes {
					body.Files = append(body.Files, callbackFile{
						Name:    name,
						S3Key:   "outputs/" + inv.ID + "/" + name,
						BlockID: inv.ID,
					})
				}
			}
		}
		if inv.LogsPath != "" {
			body.Logs = append(body.Logs, callbackLog{
				BlockID: inv.ID,
				Stdout:  inv.LogsPath,
			})
		}
	}
	return body
}
