package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// PerformanceOptimizer provides performance optimizations for pipeline operations
type PerformanceOptimizer struct {
	sessionManager *session.SessionManager
	logger         zerolog.Logger

	// Connection pooling and caching
	connectionPool map[string]interface{}
	operationCache map[string]*PerformanceCachedOperation
	cacheMutex     sync.RWMutex

	// Performance metrics
	operationMetrics map[string]*PerformanceOperationMetrics
	metricsMutex     sync.RWMutex
}

// PerformanceCachedOperation represents a cached operation result
type PerformanceCachedOperation struct {
	Key         string        `json:"key"`
	Result      interface{}   `json:"result"`
	Timestamp   time.Time     `json:"timestamp"`
	TTL         time.Duration `json:"ttl"`
	AccessCount int           `json:"access_count"`
}

// PerformanceOperationMetrics tracks performance metrics for operations
type PerformanceOperationMetrics struct {
	OperationType   string        `json:"operation_type"`
	TotalExecutions int64         `json:"total_executions"`
	SuccessfulOps   int64         `json:"successful_ops"`
	FailedOps       int64         `json:"failed_ops"`
	AverageLatency  time.Duration `json:"average_latency"`
	TotalLatency    time.Duration `json:"total_latency"`
	MinLatency      time.Duration `json:"min_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	LastExecution   time.Time     `json:"last_execution"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	CacheHits       int64         `json:"cache_hits"`
	CacheMisses     int64         `json:"cache_misses"`
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(sessionManager *session.SessionManager, logger zerolog.Logger) *PerformanceOptimizer {
	optimizer := &PerformanceOptimizer{
		sessionManager:   sessionManager,
		logger:           logger.With().Str("component", "performance_optimizer").Logger(),
		connectionPool:   make(map[string]interface{}),
		operationCache:   make(map[string]*PerformanceCachedOperation),
		operationMetrics: make(map[string]*PerformanceOperationMetrics),
	}

	// Start background cleanup goroutine
	go optimizer.startCacheCleanup()

	return optimizer
}

// OptimizeDockerOperation optimizes Docker operations with caching and pooling
func (po *PerformanceOptimizer) OptimizeDockerOperation(ctx context.Context, operationType, sessionID string, args map[string]interface{}) (interface{}, error) {
	startTime := time.Now()

	// Generate cache key
	cacheKey := po.generateCacheKey(operationType, args)

	// Check cache first
	if cachedResult := po.getCachedResult(cacheKey); cachedResult != nil {
		po.recordCacheHit(operationType)
		po.updateMetrics(operationType, startTime, true, true)

		po.logger.Debug().
			Str("operation", operationType).
			Str("cache_key", cacheKey).
			Msg("Cache hit for Docker operation")

		return cachedResult.Result, nil
	}

	po.recordCacheMiss(operationType)

	// Execute operation
	var result interface{}
	var err error

	switch operationType {
	case "pull":
		imageRef, _ := args["image_ref"].(string)
		err = po.executePullWithOptimization(ctx, sessionID, imageRef)
		result = map[string]interface{}{
			"operation": "pull",
			"image_ref": imageRef,
			"success":   err == nil,
		}
	case "push":
		imageRef, _ := args["image_ref"].(string)
		err = po.executePushWithOptimization(ctx, sessionID, imageRef)
		result = map[string]interface{}{
			"operation": "push",
			"image_ref": imageRef,
			"success":   err == nil,
		}
	case "tag":
		sourceRef, _ := args["source_ref"].(string)
		targetRef, _ := args["target_ref"].(string)
		err = po.executeTagWithOptimization(ctx, sessionID, sourceRef, targetRef)
		result = map[string]interface{}{
			"operation":  "tag",
			"source_ref": sourceRef,
			"target_ref": targetRef,
			"success":    err == nil,
		}
	default:
		err = fmt.Errorf("unsupported operation type: %s", operationType)
	}

	// Cache successful results
	if err == nil && po.shouldCache(operationType, args) {
		po.cacheResult(cacheKey, result, po.getCacheTTL(operationType))
	}

	// Update metrics
	po.updateMetrics(operationType, startTime, err == nil, false)

	return result, err
}

// BatchOptimizeOperations optimizes multiple operations in batch
func (po *PerformanceOptimizer) BatchOptimizeOperations(ctx context.Context, operations []BatchOperation) ([]interface{}, error) {
	results := make([]interface{}, len(operations))
	errors := make([]error, len(operations))

	// Use worker pool for parallel execution
	workerCount := min(len(operations), 5) // Limit concurrent operations
	jobs := make(chan int, len(operations))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jobIdx := range jobs {
				op := operations[jobIdx]
				result, err := po.OptimizeDockerOperation(ctx, op.Type, op.SessionID, op.Args)
				results[jobIdx] = result
				errors[jobIdx] = err
			}
		}()
	}

	// Send jobs
	for i := range operations {
		jobs <- i
	}
	close(jobs)

	// Wait for completion
	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			po.logger.Error().Err(err).Int("operation_index", i).Msg("Batch operation failed")
		}
	}

	return results, nil
}

// GetPerformanceMetrics returns current performance metrics
func (po *PerformanceOptimizer) GetPerformanceMetrics() map[string]*PerformanceOperationMetrics {
	po.metricsMutex.RLock()
	defer po.metricsMutex.RUnlock()

	// Create deep copy to avoid race conditions
	metrics := make(map[string]*PerformanceOperationMetrics)
	for k, v := range po.operationMetrics {
		metricsCopy := *v
		metrics[k] = &metricsCopy
	}

	return metrics
}

// Private helper methods

func (po *PerformanceOptimizer) generateCacheKey(operationType string, args map[string]interface{}) string {
	// Simple cache key generation - in production, use a more sophisticated approach
	return fmt.Sprintf("%s:%v", operationType, args)
}

func (po *PerformanceOptimizer) getCachedResult(key string) *PerformanceCachedOperation {
	po.cacheMutex.RLock()
	defer po.cacheMutex.RUnlock()

	cached, exists := po.operationCache[key]
	if !exists {
		return nil
	}

	// Check TTL
	if time.Since(cached.Timestamp) > cached.TTL {
		return nil
	}

	cached.AccessCount++
	return cached
}

func (po *PerformanceOptimizer) cacheResult(key string, result interface{}, ttl time.Duration) {
	po.cacheMutex.Lock()
	defer po.cacheMutex.Unlock()

	po.operationCache[key] = &PerformanceCachedOperation{
		Key:         key,
		Result:      result,
		Timestamp:   time.Now(),
		TTL:         ttl,
		AccessCount: 0,
	}
}

func (po *PerformanceOptimizer) shouldCache(operationType string, args map[string]interface{}) bool {
	// Only cache read operations and successful operations
	switch operationType {
	case "pull":
		return true // Pull operations can be cached
	case "push", "tag":
		return false // Write operations should not be cached
	default:
		return false
	}
}

func (po *PerformanceOptimizer) getCacheTTL(operationType string) time.Duration {
	switch operationType {
	case "pull":
		return 30 * time.Minute // Images don't change frequently
	case "push":
		return 5 * time.Minute // Shorter TTL for push operations
	case "tag":
		return 10 * time.Minute // Medium TTL for tag operations
	default:
		return 5 * time.Minute
	}
}

func (po *PerformanceOptimizer) updateMetrics(operationType string, startTime time.Time, success bool, cacheHit bool) {
	po.metricsMutex.Lock()
	defer po.metricsMutex.Unlock()

	latency := time.Since(startTime)

	metrics, exists := po.operationMetrics[operationType]
	if !exists {
		metrics = &PerformanceOperationMetrics{
			OperationType: operationType,
			MinLatency:    latency,
			MaxLatency:    latency,
		}
		po.operationMetrics[operationType] = metrics
	}

	metrics.TotalExecutions++
	metrics.LastExecution = time.Now()

	if success {
		metrics.SuccessfulOps++
	} else {
		metrics.FailedOps++
	}

	if !cacheHit {
		metrics.TotalLatency += latency
		if latency < metrics.MinLatency {
			metrics.MinLatency = latency
		}
		if latency > metrics.MaxLatency {
			metrics.MaxLatency = latency
		}

		if metrics.TotalExecutions > 0 {
			metrics.AverageLatency = metrics.TotalLatency / time.Duration(metrics.TotalExecutions)
		}
	}

	// Update cache hit rate
	if metrics.CacheHits+metrics.CacheMisses > 0 {
		metrics.CacheHitRate = float64(metrics.CacheHits) / float64(metrics.CacheHits+metrics.CacheMisses)
	}
}

func (po *PerformanceOptimizer) recordCacheHit(operationType string) {
	po.metricsMutex.Lock()
	defer po.metricsMutex.Unlock()

	if metrics, exists := po.operationMetrics[operationType]; exists {
		metrics.CacheHits++
	}
}

func (po *PerformanceOptimizer) recordCacheMiss(operationType string) {
	po.metricsMutex.Lock()
	defer po.metricsMutex.Unlock()

	if metrics, exists := po.operationMetrics[operationType]; exists {
		metrics.CacheMisses++
	}
}

func (po *PerformanceOptimizer) startCacheCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		po.cleanupExpiredCache()
	}
}

func (po *PerformanceOptimizer) cleanupExpiredCache() {
	po.cacheMutex.Lock()
	defer po.cacheMutex.Unlock()

	now := time.Now()
	var expiredKeys []string

	for key, cached := range po.operationCache {
		if now.Sub(cached.Timestamp) > cached.TTL {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(po.operationCache, key)
	}

	if len(expiredKeys) > 0 {
		po.logger.Debug().Int("expired_count", len(expiredKeys)).Msg("Cleaned up expired cache entries")
	}
}

// Optimized execution methods (placeholders for actual implementations)

func (po *PerformanceOptimizer) executePullWithOptimization(ctx context.Context, sessionID, imageRef string) error {
	// In a real implementation, this would use connection pooling, retry logic, etc.
	po.logger.Debug().Str("image_ref", imageRef).Msg("Executing optimized pull")
	return nil // Placeholder
}

func (po *PerformanceOptimizer) executePushWithOptimization(ctx context.Context, sessionID, imageRef string) error {
	// In a real implementation, this would use connection pooling, compression, etc.
	po.logger.Debug().Str("image_ref", imageRef).Msg("Executing optimized push")
	return nil // Placeholder
}

func (po *PerformanceOptimizer) executeTagWithOptimization(ctx context.Context, sessionID, sourceRef, targetRef string) error {
	// In a real implementation, this would optimize tag operations
	po.logger.Debug().Str("source_ref", sourceRef).Str("target_ref", targetRef).Msg("Executing optimized tag")
	return nil // Placeholder
}

// BatchOperation represents an operation in a batch
type BatchOperation struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id"`
	Args      map[string]interface{} `json:"args"`
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
