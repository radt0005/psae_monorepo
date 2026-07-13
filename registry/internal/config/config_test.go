package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadRegistryDefaults(t *testing.T) {
	for _, k := range []string{
		"LISTEN_ADDR", "DATABASE_URL", "S3_ENDPOINT", "REQUIRE_APPROVAL",
		"ADMIN_USER_IDS", "BUILDER_IMAGES", "BUILD_TIMEOUT", "SIGNING_KEY_SOURCE",
		"MIRROR_ENABLED", "BUILD_DISPATCH_ENABLED",
	} {
		t.Setenv(k, "")
	}
	c, err := LoadRegistry()
	require.NoError(t, err)
	require.Equal(t, ":8090", c.ListenAddr)
	require.Equal(t, "staging/", c.StagingPrefix)
	require.Equal(t, "artifacts/", c.ArtifactPrefix)
	require.False(t, c.RequireApproval)
	require.False(t, c.MirrorEnabled, "mirror off when no DATABASE_URL")
	require.True(t, c.BuildDispatchEnabled, "embedded dispatch on by default")
	require.Equal(t, "spade-builder-go:latest", c.BuilderImages["go"])
}

func TestLoadRegistryBuildDispatchDisabled(t *testing.T) {
	t.Setenv("BUILD_DISPATCH_ENABLED", "false")
	c, err := LoadRegistry()
	require.NoError(t, err)
	require.False(t, c.BuildDispatchEnabled)
}

func TestLoadRegistryParsing(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REQUIRE_APPROVAL", "true")
	t.Setenv("ADMIN_USER_IDS", "a, b ,c")
	t.Setenv("BUILDER_IMAGES", "go=img-go:1,python=img-py:2")
	t.Setenv("BUILD_TIMEOUT", "90s")
	t.Setenv("S3_USE_PATH_STYLE", "false")

	c, err := LoadRegistry()
	require.NoError(t, err)
	require.True(t, c.RequireApproval)
	require.Equal(t, []string{"a", "b", "c"}, c.AdminUserIDs)
	require.Equal(t, "img-py:2", c.BuilderImages["python"])
	require.Equal(t, 90*time.Second, c.BuildTimeout)
	require.False(t, c.S3UsePathStyle)
	require.True(t, c.MirrorEnabled, "mirror defaults on with DATABASE_URL")
	require.True(t, c.IsAdmin("b"))
	require.False(t, c.IsAdmin("z"))
}

func TestLoadRegistryEnvKeySourceRequiresKeys(t *testing.T) {
	t.Setenv("SIGNING_KEY_SOURCE", "env")
	t.Setenv("SIGNING_PUBLIC_KEY", "")
	t.Setenv("SIGNING_PRIVATE_KEY", "")
	_, err := LoadRegistry()
	require.Error(t, err)
}

func TestLoadBuilderRequiredVars(t *testing.T) {
	for _, k := range []string{"REGISTRY_URL", "BUILD_JOB_ID", "BUILD_TOKEN"} {
		t.Setenv(k, "")
	}
	_, err := LoadBuilder()
	require.Error(t, err)

	t.Setenv("REGISTRY_URL", "http://r")
	t.Setenv("BUILD_JOB_ID", "j1")
	t.Setenv("BUILD_TOKEN", "tok")
	c, err := LoadBuilder()
	require.NoError(t, err)
	require.Equal(t, "j1", c.JobID)
}
