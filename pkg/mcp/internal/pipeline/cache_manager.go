package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
)

// NewCacheManager creates a new simple cache manager
func NewCacheManager(
	sessionManager *session.SessionManager,
	config CacheConfig,
	logger zerolog.Logger,
) *CacheManager {
	if config.EvictionPolicy == "" {
		config.EvictionPolicy = "lru"
	}
	if config.MaxCacheSize == 0 {
		config.MaxCacheSize = 512 * 1024 * 1024
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 1 * time.Hour
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}
	if config.MaxEntries == 0 {
		config.MaxEntries = 10000
	}

	cm := &CacheManager{
		sessionManager: sessionManager,
		logger:         logger.With().Str("component", "cache_manager").Logger(),
		cache:          make(map[string]*CacheEntry),
		config:         config,
		shutdownCh:     make(chan struct{}),
		metrics: &CacheMetrics{
			LastUpdated: time.Now(),
		},
	}

	go cm.startCleanup()

	cm.logger.Info().
		Str("eviction_policy", config.EvictionPolicy).
		Int64("max_cache_size", config.MaxCacheSize).
		Int("max_entries", config.MaxEntries).
		Msg("Cache manager initialized")

	return cm
}

// Get retrieves a value from the cache
func (cm *CacheManager) Get(ctx context.Context, key string) (*CacheOperation, error) {
	startTime := time.Now()

	cm.cacheMutex.RLock()
	entry, exists := cm.cache[key]
	cm.cacheMutex.RUnlock()

	if exists && cm.isEntryValid(entry) {
		cm.cacheMutex.Lock()
		entry.LastAccess = time.Now()
		entry.AccessCount++
		cm.cacheMutex.Unlock()

		cm.metricsMutex.Lock()
		cm.metrics.HitCount++
		cm.metrics.LastUpdated = time.Now()
		cm.metricsMutex.Unlock()

		return &CacheOperation{
			Success:   true,
			Entry:     entry,
			Latency:   time.Since(startTime),
			Timestamp: startTime,
		}, nil
	}

	cm.metricsMutex.Lock()
	cm.metrics.MissCount++
	cm.metrics.LastUpdated = time.Now()
	cm.metricsMutex.Unlock()

	return &CacheOperation{
		Success:   false,
		Error:     "cache miss",
		Latency:   time.Since(startTime),
		Timestamp: startTime,
	}, nil
}

// Set stores a value in the cache
func (cm *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}

	now := time.Now()
	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		TTL:         ttl,
		CreatedAt:   now,
		LastAccess:  now,
		AccessCount: 0,
		Size:        cm.calculateEntrySize(value),
	}

	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()

	if len(cm.cache) >= cm.config.MaxEntries {
		cm.evictLRU()
	}

	if cm.getCurrentCacheSize()+entry.Size > cm.config.MaxCacheSize {
		cm.evictBySize(entry.Size)
	}

	cm.cache[key] = entry

	cm.metricsMutex.Lock()
	cm.metrics.SetCount++
	cm.metrics.LastUpdated = time.Now()
	cm.metricsMutex.Unlock()

	return nil
}

// Delete removes a value from the cache
func (cm *CacheManager) Delete(ctx context.Context, key string) error {
	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()

	if _, exists := cm.cache[key]; exists {
		delete(cm.cache, key)

		cm.metricsMutex.Lock()
		cm.metrics.DeleteCount++
		cm.metrics.LastUpdated = time.Now()
		cm.metricsMutex.Unlock()
	}

	return nil
}

// Clear removes all entries from the cache
func (cm *CacheManager) Clear(ctx context.Context) error {
	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()

	cm.cache = make(map[string]*CacheEntry)
	return nil
}

// Exists checks if a key exists in the cache
func (cm *CacheManager) Exists(ctx context.Context, key string) (bool, error) {
	cm.cacheMutex.RLock()
	entry, exists := cm.cache[key]
	cm.cacheMutex.RUnlock()

	if !exists {
		return false, nil
	}

	return cm.isEntryValid(entry), nil
}

// Keys returns all valid keys in the cache
func (cm *CacheManager) Keys(ctx context.Context) ([]string, error) {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	var keys []string
	for key, entry := range cm.cache {
		if cm.isEntryValid(entry) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Size returns the number of valid entries in the cache
func (cm *CacheManager) Size(ctx context.Context) (int, error) {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	count := 0
	for _, entry := range cm.cache {
		if cm.isEntryValid(entry) {
			count++
		}
	}

	return count, nil
}

// GetSize returns the total size of the cache in bytes
func (cm *CacheManager) GetSize(ctx context.Context) (int64, error) {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	return cm.getCurrentCacheSize(), nil
}

// GetMetrics returns current cache metrics
func (cm *CacheManager) GetMetrics() *CacheMetrics {
	cm.metricsMutex.RLock()
	defer cm.metricsMutex.RUnlock()

	metrics := *cm.metrics

	cm.cacheMutex.RLock()
	metrics.TotalSize = cm.getCurrentCacheSize()
	metrics.EntryCount = len(cm.cache)
	cm.cacheMutex.RUnlock()

	metrics.LastUpdated = time.Now()
	return &metrics
}

// Shutdown gracefully shuts down the cache manager
func (cm *CacheManager) Shutdown(ctx context.Context) error {
	cm.logger.Info().Msg("Shutting down cache manager")

	close(cm.shutdownCh)

	cm.logger.Info().Msg("Cache manager shutdown complete")
	return nil
}

// isEntryValid checks if a cache entry is still valid
func (cm *CacheManager) isEntryValid(entry *CacheEntry) bool {
	if entry == nil {
		return false
	}
	return time.Since(entry.CreatedAt) < entry.TTL
}

// calculateEntrySize estimates the size of a cache entry
func (cm *CacheManager) calculateEntrySize(value interface{}) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v)) + 64
	case []byte:
		return int64(len(v)) + 64
	default:
		return 256
	}
}

// getCurrentCacheSize calculates current cache size
func (cm *CacheManager) getCurrentCacheSize() int64 {
	var totalSize int64
	for _, entry := range cm.cache {
		if cm.isEntryValid(entry) {
			totalSize += entry.Size
		}
	}
	return totalSize
}

// evictLRU evicts the least recently used entry
func (cm *CacheManager) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range cm.cache {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		delete(cm.cache, oldestKey)
		cm.metricsMutex.Lock()
		cm.metrics.EvictionCount++
		cm.metricsMutex.Unlock()
	}
}

// evictBySize evicts entries to make room for new entry
func (cm *CacheManager) evictBySize(neededSize int64) {
	currentSize := cm.getCurrentCacheSize()
	targetSize := cm.config.MaxCacheSize - neededSize

	if currentSize <= targetSize {
		return
	}

	type entryWithKey struct {
		key   string
		entry *CacheEntry
	}

	var entries []entryWithKey
	for key, entry := range cm.cache {
		entries = append(entries, entryWithKey{key: key, entry: entry})
	}

	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].entry.LastAccess.After(entries[j].entry.LastAccess) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	evicted := 0
	for _, e := range entries {
		if currentSize <= targetSize {
			break
		}
		delete(cm.cache, e.key)
		currentSize -= e.entry.Size
		evicted++
	}

	if evicted > 0 {
		cm.metricsMutex.Lock()
		cm.metrics.EvictionCount += int64(evicted)
		cm.metricsMutex.Unlock()
	}
}

// startCleanup starts the background cleanup process
func (cm *CacheManager) startCleanup() {
	ticker := time.NewTicker(cm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.performCleanup()
		case <-cm.shutdownCh:
			return
		}
	}
}

// performCleanup removes expired entries
func (cm *CacheManager) performCleanup() {
	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()

	var expiredKeys []string
	for key, entry := range cm.cache {
		if !cm.isEntryValid(entry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(cm.cache, key)
	}

	if len(expiredKeys) > 0 {
		cm.metricsMutex.Lock()
		cm.metrics.EvictionCount += int64(len(expiredKeys))
		cm.metricsMutex.Unlock()

		cm.logger.Debug().Int("expired", len(expiredKeys)).Msg("Cache cleanup completed")
	}
}

// calculateChecksum calculates a checksum for the value (if needed for integrity checks)
func calculateChecksum(value interface{}) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%v", value)))
	return hex.EncodeToString(hash[:])
}
