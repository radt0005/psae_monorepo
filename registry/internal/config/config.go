// Package config loads registryd and builder configuration from the
// environment (12-factor; App Platform and docker-compose friendly). It reuses
// the S3_* variable names already established in the repo's docker-compose.
package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// RegistryConfig configures the control-plane process (cmd/registryd).
type RegistryConfig struct {
	ListenAddr  string
	DatabaseURL string // empty → SQLitePath is used (local/dev)
	SQLitePath  string

	// Blob storage (artifacts). If S3Endpoint is empty, a filesystem blob
	// store rooted at BlobDir is used (trivial local runs and tests).
	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3UsePathStyle bool
	BlobDir        string

	StagingPrefix  string
	ArtifactPrefix string

	RequireApproval bool
	AdminUserIDs    []string

	// BuildDispatchEnabled controls the in-process build dispatcher. Disable it
	// when a standalone build service (cmd/buildrunnerd) owns the queue — e.g.
	// on App Platform, which has no Docker daemon. The /builds/:id/* callback
	// endpoints stay on regardless; remote builders report over HTTP.
	BuildDispatchEnabled bool

	BuilderImages map[string]string // language → container image
	DockerHost    string
	// BuilderDockerArgs are extra `docker run` args for the build container
	// (e.g. "--network=spade_default" so it can reach the registry and S3).
	BuilderDockerArgs []string
	BuildTimeout      time.Duration

	SigningKeySource  string // "db" (default) | "env"
	SigningPublicKey  string // base64, when source=env
	SigningPrivateKey string // base64, when source=env

	// MirrorEnabled controls whether transitions push to the web_ui blocks
	// table. Defaults true when a DatabaseURL is set.
	MirrorEnabled bool
}

// BuilderConfig configures the build worker (cmd/builder) that runs inside a
// per-language container. It has no database access by design.
type BuilderConfig struct {
	RegistryURL string
	JobID       string
	Token       string

	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3UsePathStyle bool

	StagingPrefix string
	WorkDir       string
}

// LoadRegistry reads RegistryConfig from the environment, applying defaults.
func LoadRegistry() (RegistryConfig, error) {
	c := RegistryConfig{
		ListenAddr:           envOr("LISTEN_ADDR", ":8090"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		SQLitePath:           envOr("SQLITE_PATH", "registry.db"),
		S3Endpoint:           os.Getenv("S3_ENDPOINT"),
		S3Region:             envOr("S3_REGION", "us-east-1"),
		S3Bucket:             envOr("S3_BUCKET", "spade-artifacts"),
		S3AccessKey:          os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretKey:          os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3UsePathStyle:       envBool("S3_USE_PATH_STYLE", true),
		BlobDir:              envOr("BLOB_DIR", "blobs"),
		StagingPrefix:        envOr("STAGING_PREFIX", "staging/"),
		ArtifactPrefix:       envOr("ARTIFACT_PREFIX", "artifacts/"),
		RequireApproval:      envBool("REQUIRE_APPROVAL", false),
		AdminUserIDs:         splitList(os.Getenv("ADMIN_USER_IDS")),
		BuildDispatchEnabled: envBool("BUILD_DISPATCH_ENABLED", true),
		BuilderImages:        parseImages(os.Getenv("BUILDER_IMAGES")),
		DockerHost:           os.Getenv("DOCKER_HOST"),
		BuilderDockerArgs:    splitList(os.Getenv("BUILDER_DOCKER_ARGS")),
		BuildTimeout:         envDuration("BUILD_TIMEOUT", 15*time.Minute),
		SigningKeySource:     envOr("SIGNING_KEY_SOURCE", "db"),
		SigningPublicKey:     os.Getenv("SIGNING_PUBLIC_KEY"),
		SigningPrivateKey:    os.Getenv("SIGNING_PRIVATE_KEY"),
	}
	c.MirrorEnabled = envBool("MIRROR_ENABLED", c.DatabaseURL != "")
	if c.SigningKeySource == "env" && (c.SigningPublicKey == "" || c.SigningPrivateKey == "") {
		return c, fmt.Errorf("SIGNING_KEY_SOURCE=env requires SIGNING_PUBLIC_KEY and SIGNING_PRIVATE_KEY")
	}
	if len(c.BuilderImages) == 0 {
		// Sensible default for the only real builder.
		c.BuilderImages = map[string]string{"go": "spade-builder-go:latest"}
	}
	return c, nil
}

// LoadBuilder reads BuilderConfig from the environment.
func LoadBuilder() (BuilderConfig, error) {
	c := BuilderConfig{
		RegistryURL:    os.Getenv("REGISTRY_URL"),
		JobID:          os.Getenv("BUILD_JOB_ID"),
		Token:          os.Getenv("BUILD_TOKEN"),
		S3Endpoint:     os.Getenv("S3_ENDPOINT"),
		S3Region:       envOr("S3_REGION", "us-east-1"),
		S3Bucket:       envOr("S3_BUCKET", "spade-artifacts"),
		S3AccessKey:    os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3UsePathStyle: envBool("S3_USE_PATH_STYLE", true),
		StagingPrefix:  envOr("STAGING_PREFIX", "staging/"),
		WorkDir:        envOr("BUILD_WORKDIR", ""),
	}
	var missing []string
	if c.RegistryURL == "" {
		missing = append(missing, "REGISTRY_URL")
	}
	if c.JobID == "" {
		missing = append(missing, "BUILD_JOB_ID")
	}
	if c.Token == "" {
		missing = append(missing, "BUILD_TOKEN")
	}
	if len(missing) > 0 {
		return c, fmt.Errorf("builder config missing required vars: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

// IsAdmin reports whether userID is in the operator allowlist.
func (c RegistryConfig) IsAdmin(userID string) bool {
	return slices.Contains(c.AdminUserIDs, userID)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseImages parses "go=img-go:tag,python=img-py:tag" into a map.
func parseImages(s string) map[string]string {
	m := map[string]string{}
	for _, pair := range splitList(s) {
		k, v, ok := strings.Cut(pair, "=")
		if ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}
