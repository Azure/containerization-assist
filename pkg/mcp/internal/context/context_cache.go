package context

import (
	"sync"
	"time"
)

// ContextCache provides caching for comprehensive contexts
type ContextCache struct {
	cache         map[string]*cacheEntry
	ttl           time.Duration
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
}

type cacheEntry struct {
	context   *ComprehensiveContext
	expiresAt time.Time
}

// NewContextCache creates a new context cache
func NewContextCache(ttl time.Duration) *ContextCache {
	c := &ContextCache{
		cache:         make(map[string]*cacheEntry),
		ttl:           ttl,
		cleanupTicker: time.NewTicker(ttl / 2),
	}

	// Start cleanup routine
	go c.cleanupRoutine()

	return c
}

// Get retrieves a context from cache
func (c *ContextCache) Get(key string) *ComprehensiveContext {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.context
}

// Set stores a context in cache
func (c *ContextCache) Set(key string, context *ComprehensiveContext) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &cacheEntry{
		context:   context,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a context from cache
func (c *ContextCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

// Clear clears all cached contexts
func (c *ContextCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheEntry)
}

// Size returns the number of cached contexts
func (c *ContextCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// cleanupRoutine periodically removes expired entries
func (c *ContextCache) cleanupRoutine() {
	for range c.cleanupTicker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *ContextCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.After(entry.expiresAt) {
			delete(c.cache, key)
		}
	}
}

// Stop stops the cleanup routine
func (c *ContextCache) Stop() {
	c.cleanupTicker.Stop()
}
