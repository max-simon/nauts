package provider

import (
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

type cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

func newCache(ttl time.Duration) *cache {
	return &cache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// get returns the cached value for the key, or nil if not present or expired.
func (c *cache) get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.value
}

// put stores a value in the cache with the configured TTL.
func (c *cache) put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// invalidate removes a single entry from the cache.
func (c *cache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// invalidatePrefix removes all entries whose key starts with the given prefix.
func (c *cache) invalidatePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

// clear removes all entries from the cache.
func (c *cache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}
