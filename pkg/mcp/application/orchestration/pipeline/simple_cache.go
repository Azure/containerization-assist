package pipeline

import (
	"sync"
	"time"
)

// SimpleCache provides basic in-memory caching for pipeline operations
// Replaces over-engineered distributed caching system
type SimpleCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	value   interface{}
	expires time.Time
}

// NewSimpleCache creates a basic cache
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		items: make(map[string]cacheItem),
	}
}

// Get retrieves a cached value
func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists || time.Now().After(item.expires) {
		return nil, false
	}
	return item.value, true
}

// Set stores a value with TTL
func (c *SimpleCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:   value,
		expires: time.Now().Add(ttl),
	}
}

// Clear removes all cached items
func (c *SimpleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheItem)
}
