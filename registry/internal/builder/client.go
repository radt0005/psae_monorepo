package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"spade_registry/internal/wire"
)

// Client is the builder's HTTP client for the registry's builder-facing
// endpoints. It authenticates with the per-job build token.
type Client struct {
	baseURL string
	token   string
	jobID   string
	http    *http.Client
}

// NewClient builds a registry client.
func NewClient(baseURL, jobID, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		jobID:   jobID,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("registry %s %s: %d: %s", method, path, resp.StatusCode, data)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// Job fetches the build job detail.
func (c *Client) Job(ctx context.Context) (wire.BuildJobDetail, error) {
	var d wire.BuildJobDetail
	err := c.do(ctx, http.MethodGet, "/builds/"+c.jobID, nil, &d)
	return d, err
}

// ReportScreening reports the screening outcome and returns whether the builder
// should proceed to build (false when screening failed or approval is required).
func (c *Client) ReportScreening(ctx context.Context, r wire.ScreeningReport) (bool, error) {
	var ack wire.ScreeningAck
	err := c.do(ctx, http.MethodPost, "/builds/"+c.jobID+"/screening", r, &ack)
	return ack.Proceed, err
}

// Complete reports a successful build.
func (c *Client) Complete(ctx context.Context, r wire.CompleteRequest) error {
	return c.do(ctx, http.MethodPost, "/builds/"+c.jobID+"/complete", r, nil)
}

// Fail reports a failed build.
func (c *Client) Fail(ctx context.Context, r wire.FailRequest) error {
	return c.do(ctx, http.MethodPost, "/builds/"+c.jobID+"/fail", r, nil)
}
