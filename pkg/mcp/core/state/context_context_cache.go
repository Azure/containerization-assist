package core

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ContextCache provides caching for comprehensive contexts with LRU eviction
type ContextCache struct {
	cache         map[string]*cacheEntry
	lru           *lruList
	ttl           time.Duration
	maxSize       int
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	logger        zerolog.Logger
}

type cacheEntry struct {
	context   *ComprehensiveContext
	expiresAt time.Time
	key       string
	prev      *cacheEntry
	next      *cacheEntry
}

// lruList manages the LRU order
type lruList struct {
	head *cacheEntry
	tail *cacheEntry
	size int
}

// NewContextCache creates a new context cache with LRU eviction
func NewContextCache(ttl time.Duration) *ContextCache {
	return NewContextCacheWithSize(ttl, 10000, nil, zerolog.Nop())
}

// NewContextCacheWithSize creates a new context cache with specified max size
func NewContextCacheWithSize(ttl time.Duration, maxSize int, _ interface{}, logger zerolog.Logger) *ContextCache {
	ctx, cancel := context.WithCancel(context.Background())
	c := &ContextCache{
		cache:         make(map[string]*cacheEntry),
		lru:           &lruList{},
		ttl:           ttl,
		maxSize:       maxSize,
		cleanupTicker: time.NewTicker(ttl / 2),
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
		logger:        logger.With().Str("component", "context_cache").Logger(),
	}

	c.lru.head = &cacheEntry{}
	c.lru.tail = &cacheEntry{}
	c.lru.head.next = c.lru.tail
	c.lru.tail.prev = c.lru.head

	go c.cleanupRoutine()

	return c
}

// Get retrieves a context from cache
func (c *ContextCache) Get(key string) *ComprehensiveContext {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		c.removeEntry(entry)
		return nil
	}

	c.moveToFront(entry)

	// metrics recording removed

	return entry.context
}

// Set stores a context in cache
func (c *ContextCache) Set(key string, context *ComprehensiveContext) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.cache[key]; exists {
		entry.context = context
		entry.expiresAt = time.Now().Add(c.ttl)
		c.moveToFront(entry)
		return
	}

	entry := &cacheEntry{
		context:   context,
		expiresAt: time.Now().Add(c.ttl),
		key:       key,
	}

	c.cache[key] = entry
	c.addToFront(entry)

	if c.lru.size > c.maxSize {
		c.evictLRU()
	}

	// metrics recording removed
}

// Delete removes a context from cache
func (c *ContextCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.cache[key]; exists {
		c.removeEntry(entry)
	}
}

// Clear clears all cached contexts
func (c *ContextCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheEntry)
	c.lru.head.next = c.lru.tail
	c.lru.tail.prev = c.lru.head
	c.lru.size = 0
}

// Size returns the number of cached contexts
func (c *ContextCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// cleanupRoutine periodically removes expired entries
func (c *ContextCache) cleanupRoutine() {
	defer close(c.done)

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.cleanupTicker.C:
			c.cleanup()
		}
	}
}

// cleanup removes expired entries
func (c *ContextCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var toRemove []*cacheEntry

	for _, entry := range c.cache {
		if now.After(entry.expiresAt) {
			toRemove = append(toRemove, entry)
		}
	}

	for _, entry := range toRemove {
		c.removeEntry(entry)
	}
}

// Stop stops the cleanup routine and waits for it to finish
func (c *ContextCache) Stop() {
	c.cancel()
	c.cleanupTicker.Stop()
	<-c.done
}

// addToFront adds entry to the front of LRU list
func (c *ContextCache) addToFront(entry *cacheEntry) {
	entry.next = c.lru.head.next
	entry.prev = c.lru.head
	c.lru.head.next.prev = entry
	c.lru.head.next = entry
	c.lru.size++
}

// removeFromList removes entry from LRU list
func (c *ContextCache) removeFromList(entry *cacheEntry) {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
	c.lru.size--
}

// moveToFront moves entry to front of LRU list
func (c *ContextCache) moveToFront(entry *cacheEntry) {
	c.removeFromList(entry)
	c.addToFront(entry)
}

// removeEntry removes entry from both cache and LRU list
func (c *ContextCache) removeEntry(entry *cacheEntry) {
	delete(c.cache, entry.key)
	c.removeFromList(entry)
}

// evictLRU removes the least recently used entry
func (c *ContextCache) evictLRU() {
	if c.lru.size > 0 {
		lru := c.lru.tail.prev
		c.removeEntry(lru)
	}
}
