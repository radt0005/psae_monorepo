// Package wire defines the JSON request/response payloads exchanged over the
// registry HTTP API. It is shared by internal/api (server) and internal/builder
// (the build worker's client) so the contract stays in one place.
package wire

// PublishRequest is the body of POST /publish (cli.md "spade publish").
type PublishRequest struct {
	RepoURL    string `json:"repo_url"`
	CommitSHA  string `json:"commit_sha"`
	Collection string `json:"collection"`
	Version    string `json:"version"`
	Language   string `json:"language,omitempty"`
}

// PublishResponse acknowledges a publish and points at the tracking URL.
type PublishResponse struct {
	VersionID string `json:"version_id"`
	StatusURL string `json:"status_url"`
	State     string `json:"state"`
}

// BlockManifestWire is a block's manifest metadata as carried over the API
// (mirrors core.BlockManifest, decoupled from its YAML tags).
type BlockManifestWire struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Kind        string         `json:"kind,omitempty"`
	Network     bool           `json:"network,omitempty"`
	Description string         `json:"description,omitempty"`
	Entrypoint  string         `json:"entrypoint,omitempty"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	Outputs     map[string]any `json:"outputs,omitempty"`
}

// BuildJobDetail is returned by GET /builds/:id to the launched build container.
type BuildJobDetail struct {
	JobID         string `json:"job_id"`
	VersionID     string `json:"version_id"`
	Collection    string `json:"collection"`
	Version       string `json:"version"`
	RepoURL       string `json:"repo_url"`
	CommitSHA     string `json:"commit_sha"`
	Language      string `json:"language"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	StagingPrefix string `json:"staging_prefix"`
	StagingKey    string `json:"staging_key"`
	ArtifactKey   string `json:"artifact_key"`
}

// ScreeningReport is the body of POST /builds/:id/screening.
type ScreeningReport struct {
	ScreenerName    string `json:"screener_name"`
	ScreenerVersion string `json:"screener_version"`
	Passed          bool   `json:"passed"`
	Details         string `json:"details,omitempty"`
}

// ScreeningAck is the response to POST /builds/:id/screening. Proceed is false
// when screening failed or when human approval is required before building, in
// which case the builder stops without building.
type ScreeningAck struct {
	Proceed bool `json:"proceed"`
}

// CompleteRequest is the body of POST /builds/:id/complete: the builder reports
// the staged artifact location, its content hash, and the block metadata.
type CompleteRequest struct {
	Platform    string              `json:"platform"`
	Arch        string              `json:"arch"`
	ContentHash string              `json:"content_hash"`
	StagingKey  string              `json:"staging_key"`
	SizeBytes   int64               `json:"size_bytes"`
	Blocks      []BlockManifestWire `json:"blocks"`
}

// FailRequest is the body of POST /builds/:id/fail.
type FailRequest struct {
	Reason  string `json:"reason"`
	LogsKey string `json:"logs_key,omitempty"`
}

// StateChangeRequest is the body of POST /collections/:name/:version/state.
type StateChangeRequest struct {
	ToState string `json:"to_state"`
	Reason  string `json:"reason,omitempty"`
}

// CollectionInfo is one entry of GET /collections.
type CollectionInfo struct {
	Name        string `json:"name"`
	Language    string `json:"language"`
	Description string `json:"description,omitempty"`
}

// VersionInfo is one entry of GET /collections/:name/versions.
type VersionInfo struct {
	Version string `json:"version"`
	State   string `json:"state"`
}

// ArtifactInfo describes a built artifact in a version status response.
type ArtifactInfo struct {
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	ContentHash string `json:"content_hash"`
}

// ArtifactMeta is the body of GET /artifacts/:name/:version/:platform/:arch/meta,
// the worker-facing endpoint the installer reads to verify the artifact content
// hash and to observe the current version state (available/yanked/recalled). It
// returns the state unconditionally (unlike the artifact fetch, which 410s on
// non-servable states) so the recall-freshness re-check can read it.
type ArtifactMeta struct {
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	ContentHash string `json:"content_hash"`
	State       string `json:"state"`
}

// ScreeningInfo is one screening result in a version status response.
type ScreeningInfo struct {
	ScreenerName    string `json:"screener_name"`
	ScreenerVersion string `json:"screener_version"`
	Passed          bool   `json:"passed"`
	Details         string `json:"details,omitempty"`
}

// VersionStatus is the body of GET /collections/:name/:version/status.
type VersionStatus struct {
	Collection string          `json:"collection"`
	Version    string          `json:"version"`
	State      string          `json:"state"`
	Error      string          `json:"error,omitempty"`
	Screening  []ScreeningInfo `json:"screening"`
	Artifacts  []ArtifactInfo  `json:"artifacts"`
	LogsKey    string          `json:"logs_key,omitempty"`
}

// PubKeysResponse is the body of GET /pubkeys.
type PubKeysResponse struct {
	Keys []string `json:"keys"`
}

// ErrorResponse is the uniform error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}
