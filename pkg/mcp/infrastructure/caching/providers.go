package caching

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ProvideCachingServices provides all caching-related services
func ProvideCachingServices() CachingServices {
	return CachingServices{
		WorkflowCache: ProvideWorkflowCache(),
	}
}

// CachingServices bundles all caching services
type CachingServices struct {
	WorkflowCache *WorkflowCache
}

// ProvideWorkflowCache provides a workflow cache instance
func ProvideWorkflowCache() *WorkflowCache {
	return NewWorkflowCache()
}

// ProvideMemoryCache provides a generic memory cache
func ProvideMemoryCache(maxSize int, cleanupInterval time.Duration) *MemoryCache {
	return NewMemoryCache(maxSize, cleanupInterval)
}

// ProvideLayeredCache provides a layered cache with multiple tiers
func ProvideLayeredCache() *LayeredCache {
	// L1: Hot cache - small, fast
	l1 := NewMemoryCache(100, 30*time.Second)

	// L2: Warm cache - larger, slightly slower
	l2 := NewMemoryCache(1000, 5*time.Minute)

	return NewLayeredCache(l1, l2)
}

// ProvideCachedOrchestrator provides a workflow orchestrator with caching
func ProvideCachedOrchestrator(base workflow.WorkflowOrchestrator, cache *WorkflowCache) workflow.WorkflowOrchestrator {
	return NewWorkflowCacheDecorator(base, cache)
}
