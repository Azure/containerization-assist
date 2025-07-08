package state

import (
	"time"
)

// Note: RegisterContextProvider method is implemented in state_types.go with variadic arguments
// This allows both single provider and named provider registration patterns

// Field to add to AIContextAggregator struct (this is just documentation)
// namedProviders map[string]ContextProvider

// Note: GetComprehensiveContext is already implemented in state_types.go
// This file only contains the overloaded RegisterContextProvider method

// Set method for ContextCache
func (c *ContextCache) Set(sessionID string, context *ComprehensiveContext) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict old entries if at capacity
	if len(c.cache) >= c.maxSize {
		// Simple eviction: remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.cache {
			if oldestKey == "" || v.UpdatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.UpdatedAt
			}
		}
		if oldestKey != "" {
			delete(c.cache, oldestKey)
		}
	}

	context.UpdatedAt = time.Now()
	c.cache[sessionID] = context
}
