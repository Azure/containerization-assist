package appstate

import (
	"log/slog"
	"sync"
	"time"
)

// ContextCacheImpl implements the ContextCache interface
type ContextCacheImpl struct {
	cache  map[string]*CacheEntry
	mu     sync.RWMutex
	ttl    time.Duration
	logger *slog.Logger
}

type CacheEntry struct {
	Data      *ComprehensiveContext
	ExpiresAt time.Time
}

func NewContextCache(ttl time.Duration, logger *slog.Logger) *ContextCacheImpl {
	cache := &ContextCacheImpl{
		cache:  make(map[string]*CacheEntry),
		ttl:    ttl,
		logger: logger,
	}

	// Start cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

func (c *ContextCacheImpl) Get(key string) (*ComprehensiveContext, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Data, true
}

func (c *ContextCacheImpl) Set(key string, data *ComprehensiveContext) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
	}

	c.logger.Debug("Context cached",
		slog.String("key", key),
		slog.Duration("ttl", c.ttl))
}

func (c *ContextCacheImpl) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

func (c *ContextCacheImpl) cleanupExpired() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.ExpiresAt) {
				delete(c.cache, key)
				c.logger.Debug("Expired context removed",
					slog.String("key", key))
			}
		}
		c.mu.Unlock()
	}
}
