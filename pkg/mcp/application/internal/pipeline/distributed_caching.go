package pipeline

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/rs/zerolog"
)

// CacheInterface defines the cache operations interface
type CacheInterface interface {
	Get(ctx context.Context, key string) (*CacheOperation, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Exists(ctx context.Context, key string) (bool, error)
	Keys(ctx context.Context) ([]string, error)
	Size(ctx context.Context) (int, error)
	GetSize(ctx context.Context) (int64, error)
	GetMetrics() *CacheMetrics
	Shutdown(ctx context.Context) error
}

// NewSimpleCache creates a new simple cache manager
func NewSimpleCache(
	sessionManager *session.SessionManager,
	config CacheConfig,
	logger zerolog.Logger,
) CacheInterface {
	return NewCacheManager(sessionManager, config, logger)
}

// NewDistributedCacheManager creates a new cache manager
func NewDistributedCacheManager(
	sessionManager *session.SessionManager,
	config CacheConfig,
	logger zerolog.Logger,
) CacheInterface {
	logger.Warn().Msg("NewDistributedCacheManager is deprecated, use NewSimpleCache instead")
	return NewCacheManager(sessionManager, config, logger)
}

// DistributedCacheManager is an alias for CacheManager
type DistributedCacheManager = CacheManager

// DistributedCacheConfig is an alias for CacheConfig
type DistributedCacheConfig = CacheConfig

// DistributedCacheMetrics is an alias for CacheMetrics
type DistributedCacheMetrics = CacheMetrics
