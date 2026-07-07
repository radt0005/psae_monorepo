package spade

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Secrets are delivered as the SPADE_SECRETS environment variable — a JSON
// object mapping the block's logical secret names to their values — by the
// worker (cloud) or CLI (local). This file parses that blob, serves values
// through GetSecret, and scrubs the variable from the environment so it is not
// inherited by any subprocess the block spawns. See spec/secrets.md §4.

var (
	secretsMu    sync.Mutex
	secretsCache map[string]string
)

func parseSecrets(raw string) map[string]string {
	m := map[string]string{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &m)
	}
	return m
}

// loadSecrets parses and caches SPADE_SECRETS, removing it from the environment
// on first read. Idempotent.
func loadSecrets() map[string]string {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	if secretsCache == nil {
		raw, ok := os.LookupEnv("SPADE_SECRETS")
		if ok {
			_ = os.Unsetenv("SPADE_SECRETS")
		}
		secretsCache = parseSecrets(raw)
	}
	return secretsCache
}

// GetSecret returns the secret bound to a logical name for this block. The
// mapping from logical name to a stored secret is declared in the pipeline
// (spec/secrets.md §3.2); the value is injected by the worker (cloud) or CLI
// (local). Returns an error if the name was not provided.
func GetSecret(name string) (string, error) {
	v, ok := loadSecrets()[name]
	if !ok {
		return "", fmt.Errorf("secret %q was not provided to this block; "+
			"declare it in the pipeline's secrets mapping", name)
	}
	return v, nil
}
