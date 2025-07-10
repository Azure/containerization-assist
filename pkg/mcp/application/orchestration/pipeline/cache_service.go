package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// NewCacheManager creates a new simple cache manager
func NewCacheManager(
	sessionManager *session.SessionManager,
	config CacheConfig,
	logger logging.Standards,
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
		logger:         logger.WithComponent("cache_manager"),
		cache:          make(map[string]*CacheEntry),
		config:         config,
		shutdownCh:     make(chan struct{}),
		metrics: &CacheMetrics{
			LastUpdated: time.Now(),
		},
	}

	go cm.startCleanup()

	return cm
}

// calculateChecksum calculates a checksum for the value (if needed for integrity checks)
func calculateChecksum(value interface{}) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%v", value)))
	return hex.EncodeToString(hash[:])
}
