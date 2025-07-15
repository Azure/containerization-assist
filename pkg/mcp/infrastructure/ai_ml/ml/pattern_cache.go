// Package ml provides pattern caching for improved performance
package ml

import (
	"sync"
	"time"
)

// PatternCache provides in-memory caching for error pattern analysis
type PatternCache struct {
	cache   map[string]*CacheEntry
	mu      sync.RWMutex
	maxSize int
	ttl     time.Duration
	hits    int64
	misses  int64
}

// CacheEntry represents a cached pattern analysis result
type CacheEntry struct {
	Classification *EnhancedErrorClassification
	Timestamp      time.Time
	AccessCount    int64
}

// NewPatternCache creates a new pattern cache
func NewPatternCache() *PatternCache {
	return &PatternCache{
		cache:   make(map[string]*CacheEntry),
		maxSize: 1000,
		ttl:     30 * time.Minute,
	}
}

// Get retrieves a cached pattern analysis
func (c *PatternCache) Get(errorPattern string) *EnhancedErrorClassification {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[errorPattern]
	if !exists {
		c.misses++
		return nil
	}

	// Check if entry has expired
	if time.Since(entry.Timestamp) > c.ttl {
		c.misses++
		// Remove expired entry
		delete(c.cache, errorPattern)
		return nil
	}

	// Update access count
	entry.AccessCount++
	c.hits++

	return entry.Classification
}

// Set stores a pattern analysis in the cache
func (c *PatternCache) Set(errorPattern string, classification *EnhancedErrorClassification) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	c.cache[errorPattern] = &CacheEntry{
		Classification: classification,
		Timestamp:      time.Now(),
		AccessCount:    1,
	}
}

// ClearPattern removes a specific pattern from the cache
func (c *PatternCache) ClearPattern(errorPattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, errorPattern)
}

// Clear removes all entries from the cache
func (c *PatternCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry)
}

// GetHitRate returns the cache hit rate
func (c *PatternCache) GetHitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0.0
	}

	return float64(c.hits) / float64(total)
}

// GetSize returns the current cache size
func (c *PatternCache) GetSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// GetStats returns cache statistics
func (c *PatternCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:    len(c.cache),
		Hits:    c.hits,
		Misses:  c.misses,
		HitRate: c.GetHitRate(),
		MaxSize: c.maxSize,
		TTL:     c.ttl,
	}
}

// evictOldest removes the oldest entry from the cache
func (c *PatternCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	first := true
	for key, entry := range c.cache {
		if first || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
			first = false
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// CacheStats provides statistics about the cache
type CacheStats struct {
	Size    int           `json:"size"`
	Hits    int64         `json:"hits"`
	Misses  int64         `json:"misses"`
	HitRate float64       `json:"hit_rate"`
	MaxSize int           `json:"max_size"`
	TTL     time.Duration `json:"ttl"`
}
