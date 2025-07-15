// Package caching provides caching infrastructure for Container Kit
package caching

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheEntry represents a single cache entry with metadata
type CacheEntry struct {
	Key        string
	Value      interface{}
	CreatedAt  time.Time
	ExpiresAt  time.Time
	AccessedAt time.Time
	HitCount   int
}

// Cache provides a generic caching interface
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) (interface{}, bool)

	// Set stores a value in the cache with expiration
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration)

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string)

	// Clear removes all entries from the cache
	Clear(ctx context.Context)

	// Stats returns cache statistics
	Stats() CacheStats
}

// CacheStats provides cache performance statistics
type CacheStats struct {
	Hits      int64
	Misses    int64
	Sets      int64
	Deletes   int64
	Evictions int64
	Size      int
	MaxSize   int
}

// MemoryCache implements an in-memory cache with TTL support
type MemoryCache struct {
	mu              sync.RWMutex
	entries         map[string]*CacheEntry
	maxSize         int
	stats           CacheStats
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int, cleanupInterval time.Duration) *MemoryCache {
	cache := &MemoryCache{
		entries:         make(map[string]*CacheEntry),
		maxSize:         maxSize,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}
	cache.stats.MaxSize = maxSize

	// Start cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.incrementMisses()
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.Delete(ctx, key)
		c.incrementMisses()
		return nil, false
	}

	// Update access time and hit count
	c.mu.Lock()
	entry.AccessedAt = time.Now()
	entry.HitCount++
	c.mu.Unlock()

	c.incrementHits()
	return entry.Value, true
}

// Set stores a value in the cache with expiration
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	now := time.Now()
	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
		AccessedAt: now,
		HitCount:   0,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries
	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	c.entries[key] = entry
	c.stats.Sets++
	c.stats.Size = len(c.entries)
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; exists {
		delete(c.entries, key)
		c.stats.Deletes++
		c.stats.Size = len(c.entries)
	}
}

// Clear removes all entries from the cache
func (c *MemoryCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.stats.Size = 0
}

// Stats returns cache statistics
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.Size = len(c.entries)
	return stats
}

// Stop stops the cleanup goroutine
func (c *MemoryCache) Stop() {
	close(c.stopCleanup)
}

// cleanupExpired periodically removes expired entries
func (c *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// removeExpired removes all expired entries
func (c *MemoryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			c.stats.Evictions++
		}
	}
	c.stats.Size = len(c.entries)
}

// evictLRU evicts the least recently used entry
func (c *MemoryCache) evictLRU() {
	var lruKey string
	var lruTime time.Time

	for key, entry := range c.entries {
		if lruTime.IsZero() || entry.AccessedAt.Before(lruTime) {
			lruKey = key
			lruTime = entry.AccessedAt
		}
	}

	if lruKey != "" {
		delete(c.entries, lruKey)
		c.stats.Evictions++
	}
}

// incrementHits safely increments hit count
func (c *MemoryCache) incrementHits() {
	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()
}

// incrementMisses safely increments miss count
func (c *MemoryCache) incrementMisses() {
	c.mu.Lock()
	c.stats.Misses++
	c.mu.Unlock()
}

// LayeredCache provides a multi-layer cache with fallback
type LayeredCache struct {
	layers []Cache
}

// NewLayeredCache creates a new layered cache
func NewLayeredCache(layers ...Cache) *LayeredCache {
	return &LayeredCache{
		layers: layers,
	}
}

// Get retrieves a value from the cache layers
func (lc *LayeredCache) Get(ctx context.Context, key string) (interface{}, bool) {
	for i, cache := range lc.layers {
		if value, found := cache.Get(ctx, key); found {
			// Populate higher layers
			for j := 0; j < i; j++ {
				lc.layers[j].Set(ctx, key, value, 5*time.Minute) // Default TTL for promotion
			}
			return value, true
		}
	}
	return nil, false
}

// Set stores a value in all cache layers
func (lc *LayeredCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	for _, cache := range lc.layers {
		cache.Set(ctx, key, value, ttl)
	}
}

// Delete removes a value from all cache layers
func (lc *LayeredCache) Delete(ctx context.Context, key string) {
	for _, cache := range lc.layers {
		cache.Delete(ctx, key)
	}
}

// Clear removes all entries from all cache layers
func (lc *LayeredCache) Clear(ctx context.Context) {
	for _, cache := range lc.layers {
		cache.Clear(ctx)
	}
}

// Stats returns aggregated statistics from all layers
func (lc *LayeredCache) Stats() CacheStats {
	stats := CacheStats{}
	for _, cache := range lc.layers {
		layerStats := cache.Stats()
		stats.Hits += layerStats.Hits
		stats.Misses += layerStats.Misses
		stats.Sets += layerStats.Sets
		stats.Deletes += layerStats.Deletes
		stats.Evictions += layerStats.Evictions
		stats.Size += layerStats.Size
		stats.MaxSize += layerStats.MaxSize
	}
	return stats
}

// CacheKey generates a cache key from components
func CacheKey(components ...string) string {
	key := ""
	for i, comp := range components {
		if i > 0 {
			key += ":"
		}
		key += comp
	}
	return key
}

// SerializableCache wraps a cache to handle JSON serialization
type SerializableCache struct {
	cache Cache
}

// NewSerializableCache creates a cache that handles JSON serialization
func NewSerializableCache(cache Cache) *SerializableCache {
	return &SerializableCache{cache: cache}
}

// Get retrieves and deserializes a value from the cache
func (sc *SerializableCache) Get(ctx context.Context, key string, target interface{}) error {
	value, found := sc.cache.Get(ctx, key)
	if !found {
		return fmt.Errorf("cache miss for key: %s", key)
	}

	// If value is already the correct type, return it
	if data, ok := value.([]byte); ok {
		return json.Unmarshal(data, target)
	}

	// Otherwise, try to marshal and unmarshal
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cached value: %w", err)
	}

	return json.Unmarshal(data, target)
}

// Set serializes and stores a value in the cache
func (sc *SerializableCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value: %w", err)
	}

	sc.cache.Set(ctx, key, data, ttl)
	return nil
}
