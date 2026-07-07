// Package kmsclient is the worker's HTTP client for the KMS resolve endpoint
// (spec/secrets.md §6.2). The worker exchanges the job's capability token for
// the values of the secrets the invocation's block declared.
package kmsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls the KMS /resolve endpoint.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client for the KMS at baseURL.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

type resolveRequest struct {
	Token string   `json:"token"`
	Names []string `json:"names"`
}

// Resolve exchanges a capability token for the values of the named (stored)
// secrets. Returns a map of stored-secret name to value.
func (c *Client) Resolve(ctx context.Context, token string, names []string) (map[string]string, error) {
	body, err := json.Marshal(resolveRequest{Token: token, Names: names})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/resolve", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("kms resolve: status %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}

	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding resolve response: %w", err)
	}
	return out, nil
}
