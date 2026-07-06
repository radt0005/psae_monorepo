// Package installer implements the worker's registry-fetch install path: pull a
// signed collection artifact from the cloud Plugin Registry, verify its signature
// and content hash, unpack it, and register its blocks in the local index — with
// no build toolchains on the worker (worker.md §Worker Installer, registry.md §8).
package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client talks to the registry's worker-facing endpoints using a rotated service
// token (Authorization: Bearer <token>).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient builds a worker registry client. A nil http.Client uses the default.
func NewClient(baseURL, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: hc}
}

// artifactMeta mirrors the registry's wire.ArtifactMeta JSON. The runner module
// cannot import the registry's wire package, so the shape is duplicated; the JSON
// contract is the coupling point (registry api.artifactMeta).
type artifactMeta struct {
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	ContentHash string `json:"content_hash"`
	State       string `json:"state"`
}

// httpError carries a non-2xx status so callers can classify it (404/410 are
// permanent rejections; 5xx and transport errors are transient/infra).
type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("registry returned %d: %s", e.Status, e.Body) }

// get issues an authenticated GET and returns the body on 2xx, else an
// *httpError (non-2xx) or the transport error.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, &httpError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	return body, nil
}

func (c *Client) fetchMeta(ctx context.Context, collection, version, platform, arch string) (artifactMeta, error) {
	body, err := c.get(ctx, fmt.Sprintf("/artifacts/%s/%s/%s/%s/meta", collection, version, platform, arch))
	if err != nil {
		return artifactMeta{}, err
	}
	var m artifactMeta
	if err := json.Unmarshal(body, &m); err != nil {
		return artifactMeta{}, fmt.Errorf("decoding artifact meta: %w", err)
	}
	return m, nil
}

func (c *Client) fetchTarball(ctx context.Context, collection, version, platform, arch string) ([]byte, error) {
	return c.get(ctx, fmt.Sprintf("/artifacts/%s/%s/%s/%s.tar.gz", collection, version, platform, arch))
}

func (c *Client) fetchSig(ctx context.Context, collection, version, platform, arch string) ([]byte, error) {
	return c.get(ctx, fmt.Sprintf("/artifacts/%s/%s/%s/%s.sig", collection, version, platform, arch))
}

// fetchPubkeys reads the trusted public key set from /pubkeys.
func (c *Client) fetchPubkeys(ctx context.Context) ([]string, error) {
	body, err := c.get(ctx, "/pubkeys")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding pubkeys: %w", err)
	}
	return resp.Keys, nil
}
