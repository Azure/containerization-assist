package pipeline

import (
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/rs/zerolog"
)

// CacheManager provides simple in-memory caching functionality
type CacheManager struct {
	sessionManager *session.SessionManager
	logger         zerolog.Logger

	cache      map[string]*CacheEntry
	cacheMutex sync.RWMutex

	config CacheConfig

	metrics      *CacheMetrics
	metricsMutex sync.RWMutex

	shutdownCh chan struct{}
}

// CacheConfig configures caching behavior
type CacheConfig struct {
	EvictionPolicy  string        `json:"eviction_policy"`
	MaxCacheSize    int64         `json:"max_cache_size"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	MaxEntries      int           `json:"max_entries"`
}

// CacheEntry represents a single cache entry
type CacheEntry struct {
	Key         string        `json:"key"`
	Value       interface{}   `json:"value"`
	TTL         time.Duration `json:"ttl"`
	CreatedAt   time.Time     `json:"created_at"`
	LastAccess  time.Time     `json:"last_access"`
	AccessCount int64         `json:"access_count"`
	Size        int64         `json:"size"`
}

// CacheMetrics tracks cache performance metrics
type CacheMetrics struct {
	HitCount       int64         `json:"hit_count"`
	MissCount      int64         `json:"miss_count"`
	SetCount       int64         `json:"set_count"`
	DeleteCount    int64         `json:"delete_count"`
	EvictionCount  int64         `json:"eviction_count"`
	AverageLatency time.Duration `json:"average_latency"`
	TotalSize      int64         `json:"total_size"`
	EntryCount     int           `json:"entry_count"`
	LastUpdated    time.Time     `json:"last_updated"`
}

// CacheOperation represents the result of a cache operation
type CacheOperation struct {
	Success   bool          `json:"success"`
	Entry     *CacheEntry   `json:"entry,omitempty"`
	Error     string        `json:"error,omitempty"`
	Latency   time.Duration `json:"latency"`
	Timestamp time.Time     `json:"timestamp"`
}
