package cmd

import (
	"os"
	"path/filepath"
)

// SpadeDir returns the root Spade directory (~/.spade/ or $SPADE_DIR).
func SpadeDir() string {
	if dir := os.Getenv("SPADE_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".spade")
	}
	return filepath.Join(home, ".spade")
}

// BlocksDir returns the blocks installation directory.
func BlocksDir() string {
	return filepath.Join(SpadeDir(), "blocks")
}

// CacheDir returns the cache directory.
func CacheDir() string {
	return filepath.Join(SpadeDir(), "cache")
}

// PipelinesDir returns the pipelines working directory.
func PipelinesDir() string {
	return filepath.Join(SpadeDir(), "pipelines")
}

// RegistryPath returns the path to the block registry database.
func RegistryPath() string {
	return filepath.Join(SpadeDir(), "registry.db")
}

// AuthDir returns the directory that holds stored credentials.
func AuthDir() string {
	return filepath.Join(SpadeDir(), "auth")
}

// CredentialsPath returns the path to the stored credentials file.
func CredentialsPath() string {
	return filepath.Join(AuthDir(), "credentials.json")
}
