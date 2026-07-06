package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PubKeyCache is a PubKeySource backed by the registry's /pubkeys endpoint, with
// an in-memory copy and a disk cache so a registry blip does not stall signature
// verification. It holds the full trusted-key *list*, so a key rotation is
// flag-day-free: during a rotation both old and new keys are served and any one
// verifying is enough (registry.md §6.1, core.VerifySignature).
type PubKeyCache struct {
	client *Client
	path   string // on-disk cache file (may be empty to disable persistence)

	mu   sync.RWMutex
	keys []string
}

// NewPubKeyCache builds a cache. cachePath persists the last good key set; pass
// "" to keep the cache in memory only.
func NewPubKeyCache(client *Client, cachePath string) *PubKeyCache {
	return &PubKeyCache{client: client, path: cachePath}
}

// Keys returns the trusted keys, preferring the in-memory copy, then the disk
// cache, then a live fetch. It avoids the network once warm.
func (c *PubKeyCache) Keys(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	keys := c.keys
	c.mu.RUnlock()
	if len(keys) > 0 {
		return keys, nil
	}
	if disk := c.loadFromDisk(); len(disk) > 0 {
		c.store(disk)
		return disk, nil
	}
	return c.Refresh(ctx)
}

// Refresh fetches the current key set from the registry and updates both caches.
// On a fetch failure it falls back to any cached keys (memory then disk) so
// verification keeps working through a registry outage; it only errors when no
// cached keys exist at all.
func (c *PubKeyCache) Refresh(ctx context.Context) ([]string, error) {
	keys, err := c.client.fetchPubkeys(ctx)
	if err != nil {
		c.mu.RLock()
		cached := c.keys
		c.mu.RUnlock()
		if len(cached) > 0 {
			return cached, nil
		}
		if disk := c.loadFromDisk(); len(disk) > 0 {
			c.store(disk)
			return disk, nil
		}
		return nil, err
	}
	c.store(keys)
	c.persist(keys)
	return keys, nil
}

// StartRefresh runs Refresh on a ticker until ctx is cancelled, keeping the cache
// current so rotations and revocations propagate without a restart.
func (c *PubKeyCache) StartRefresh(ctx context.Context, every time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_, _ = c.Refresh(ctx)
			}
		}
	}()
}

func (c *PubKeyCache) store(keys []string) {
	c.mu.Lock()
	c.keys = keys
	c.mu.Unlock()
}

func (c *PubKeyCache) loadFromDisk() []string {
	if c.path == "" {
		return nil
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil
	}
	return keys
}

func (c *PubKeyCache) persist(keys []string) {
	if c.path == "" {
		return
	}
	data, err := json.Marshal(keys)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return
	}
	// Write atomically so a crash mid-write never leaves a truncated cache.
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, c.path)
}
