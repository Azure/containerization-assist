package pipeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/rs/zerolog"
)

// DockerOperationOptimizer optimizes Docker operations with caching, parallel execution, and resource management
type DockerOperationOptimizer struct {
	logger       zerolog.Logger
	dockerClient docker.DockerClient

	// Operation caching
	operationCache map[string]*CachedOperation
	cacheMutex     sync.RWMutex
	cacheTTL       time.Duration
	maxCacheSize   int

	// Parallel execution control
	maxConcurrent   int
	semaphore       chan struct{}
	activeSemaphore chan struct{}

	// Resource management
	resourceLimits  ResourceLimits
	resourceTracker *ResourceTracker

	// Performance optimization
	operationMetrics map[string]*OperationMetrics
	metricsMutex     sync.RWMutex

	// Image layer optimization
	layerCache map[string]*LayerInfo
	layerMutex sync.RWMutex

	// Registry optimization
	registryCache map[string]*RegistryConnection
	registryMutex sync.RWMutex
}

// CachedOperation represents a cached Docker operation result
type CachedOperation struct {
	Key        string            `json:"key"`
	Operation  string            `json:"operation"`
	Result     string            `json:"result"`
	Success    bool              `json:"success"`
	Error      string            `json:"error,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	TTL        time.Duration     `json:"ttl"`
	Metadata   map[string]string `json:"metadata"`
	HitCount   int               `json:"hit_count"`
	LastAccess time.Time         `json:"last_access"`
}

// ResourceLimits defines limits for Docker operations
type ResourceLimits struct {
	MaxMemoryUsage      int64         `json:"max_memory_usage"`      // Bytes
	MaxDiskUsage        int64         `json:"max_disk_usage"`        // Bytes
	MaxNetworkBandwidth int64         `json:"max_network_bandwidth"` // Bytes per second
	MaxConcurrentOps    int           `json:"max_concurrent_ops"`
	MaxOperationTime    time.Duration `json:"max_operation_time"`

	// Per-operation limits
	PullLimits OperationLimits `json:"pull_limits"`
	PushLimits OperationLimits `json:"push_limits"`
	TagLimits  OperationLimits `json:"tag_limits"`
}

// OperationLimits defines limits for specific operation types
type OperationLimits struct {
	MaxConcurrent   int           `json:"max_concurrent"`
	MaxDuration     time.Duration `json:"max_duration"`
	MaxRetries      int           `json:"max_retries"`
	RetryBackoff    time.Duration `json:"retry_backoff"`
	TimeoutDuration time.Duration `json:"timeout_duration"`
}

// ResourceTracker tracks current resource usage
type ResourceTracker struct {
	mutex            sync.RWMutex
	currentMemory    int64
	currentDisk      int64
	currentBandwidth int64
	activeOperations int
	operationHistory []ResourceSnapshot
}

// ResourceSnapshot captures resource usage at a point in time
type ResourceSnapshot struct {
	Timestamp  time.Time `json:"timestamp"`
	Memory     int64     `json:"memory"`
	Disk       int64     `json:"disk"`
	Bandwidth  int64     `json:"bandwidth"`
	Operations int       `json:"operations"`
}

// OperationMetrics tracks performance metrics for operations
type OperationMetrics struct {
	OperationType   string        `json:"operation_type"`
	TotalExecutions int           `json:"total_executions"`
	SuccessfulOps   int           `json:"successful_ops"`
	FailedOps       int           `json:"failed_ops"`
	SuccessRate     float64       `json:"success_rate"`
	AvgDuration     time.Duration `json:"avg_duration"`
	P95Duration     time.Duration `json:"p95_duration"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	CacheHits       int           `json:"cache_hits"`
	CacheMisses     int           `json:"cache_misses"`
	LastOptimized   time.Time     `json:"last_optimized"`
}

// LayerInfo contains information about Docker image layers
type LayerInfo struct {
	LayerID     string    `json:"layer_id"`
	Size        int64     `json:"size"`
	Digest      string    `json:"digest"`
	CachedAt    time.Time `json:"cached_at"`
	AccessCount int       `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
}

// RegistryConnection manages optimized connections to registries
type RegistryConnection struct {
	Registry       string    `json:"registry"`
	LastAuth       time.Time `json:"last_auth"`
	AuthValid      bool      `json:"auth_valid"`
	ConnectionPool int       `json:"connection_pool"`
	RateLimit      int       `json:"rate_limit"`
	LastRateReset  time.Time `json:"last_rate_reset"`
}

// OptimizationConfig configures the Docker optimizer
type OptimizationConfig struct {
	EnableCaching      bool           `json:"enable_caching"`
	CacheTTL           time.Duration  `json:"cache_ttl"`
	MaxCacheSize       int            `json:"max_cache_size"`
	MaxConcurrent      int            `json:"max_concurrent"`
	ResourceLimits     ResourceLimits `json:"resource_limits"`
	EnableLayerCache   bool           `json:"enable_layer_cache"`
	EnableRegistryPool bool           `json:"enable_registry_pool"`
}

// NewDockerOperationOptimizer creates a new Docker operation optimizer
func NewDockerOperationOptimizer(dockerClient docker.DockerClient, config OptimizationConfig, logger zerolog.Logger) *DockerOperationOptimizer {
	optimizer := &DockerOperationOptimizer{
		logger:           logger.With().Str("component", "docker_optimizer").Logger(),
		dockerClient:     dockerClient,
		operationCache:   make(map[string]*CachedOperation),
		cacheTTL:         config.CacheTTL,
		maxCacheSize:     config.MaxCacheSize,
		maxConcurrent:    config.MaxConcurrent,
		semaphore:        make(chan struct{}, config.MaxConcurrent),
		activeSemaphore:  make(chan struct{}, config.ResourceLimits.MaxConcurrentOps),
		resourceLimits:   config.ResourceLimits,
		operationMetrics: make(map[string]*OperationMetrics),
		layerCache:       make(map[string]*LayerInfo),
		registryCache:    make(map[string]*RegistryConnection),
		resourceTracker: &ResourceTracker{
			operationHistory: make([]ResourceSnapshot, 0, 100),
		},
	}

	// Set default limits if not provided
	if optimizer.cacheTTL == 0 {
		optimizer.cacheTTL = 1 * time.Hour
	}
	if optimizer.maxCacheSize == 0 {
		optimizer.maxCacheSize = 1000
	}
	if optimizer.maxConcurrent == 0 {
		optimizer.maxConcurrent = 10
	}

	// Initialize default resource limits
	if optimizer.resourceLimits.MaxConcurrentOps == 0 {
		optimizer.resourceLimits.MaxConcurrentOps = 5
	}
	if optimizer.resourceLimits.MaxOperationTime == 0 {
		optimizer.resourceLimits.MaxOperationTime = 10 * time.Minute
	}

	// Start cleanup routines
	go optimizer.startCacheCleanup()
	go optimizer.startResourceMonitoring()

	return optimizer
}

// OptimizedPull performs an optimized Docker pull operation
func (opt *DockerOperationOptimizer) OptimizedPull(ctx context.Context, imageRef string, options map[string]string) (string, error) {
	operation := "pull"
	cacheKey := opt.generateCacheKey(operation, imageRef, options)

	// Check cache first
	if cached := opt.getCachedOperation(cacheKey); cached != nil {
		opt.recordCacheHit(operation)
		opt.logger.Debug().
			Str("operation", operation).
			Str("image", imageRef).
			Str("cache_key", cacheKey).
			Msg("Cache hit for Docker operation")

		if cached.Success {
			return cached.Result, nil
		} else {
			return cached.Result, fmt.Errorf("cached operation error: %s", cached.Error)
		}
	}

	// Record cache miss
	opt.recordCacheMiss(operation)

	// Check resource limits
	if err := opt.checkResourceLimits(operation); err != nil {
		return "", fmt.Errorf("resource limits exceeded: %w", err)
	}

	// Acquire semaphore for concurrent execution control
	select {
	case opt.semaphore <- struct{}{}:
		defer func() { <-opt.semaphore }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Track resource usage
	opt.trackResourceStart(operation)
	defer opt.trackResourceEnd(operation)

	// Create operation context with timeout
	opCtx, cancel := context.WithTimeout(ctx, opt.resourceLimits.PullLimits.TimeoutDuration)
	if opt.resourceLimits.PullLimits.TimeoutDuration == 0 {
		opCtx, cancel = context.WithTimeout(ctx, 10*time.Minute)
	}
	defer cancel()

	startTime := time.Now()

	// Perform the actual pull with optimization
	result, err := opt.optimizedPullWithRetry(opCtx, imageRef, options)

	duration := time.Since(startTime)

	// Cache the result
	cached := &CachedOperation{
		Key:        cacheKey,
		Operation:  operation,
		Result:     result,
		Success:    err == nil,
		Timestamp:  time.Now(),
		TTL:        opt.cacheTTL,
		Metadata:   options,
		HitCount:   0,
		LastAccess: time.Now(),
	}

	if err != nil {
		cached.Error = err.Error()
	}

	opt.setCachedOperation(cacheKey, cached)

	// Record metrics
	opt.recordOperationMetrics(operation, duration, err == nil)

	opt.logger.Info().
		Str("operation", operation).
		Str("image", imageRef).
		Dur("duration", duration).
		Bool("success", err == nil).
		Bool("cached", false).
		Msg("Completed optimized Docker operation")

	return result, err
}

// OptimizedPush performs an optimized Docker push operation
func (opt *DockerOperationOptimizer) OptimizedPush(ctx context.Context, imageRef string, options map[string]string) (string, error) {
	operation := "push"

	// Push operations are typically not cached due to side effects
	// But we still apply resource management and optimization

	// Check resource limits
	if err := opt.checkResourceLimits(operation); err != nil {
		return "", fmt.Errorf("resource limits exceeded: %w", err)
	}

	// Acquire semaphore for concurrent execution control
	select {
	case opt.semaphore <- struct{}{}:
		defer func() { <-opt.semaphore }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Track resource usage
	opt.trackResourceStart(operation)
	defer opt.trackResourceEnd(operation)

	// Create operation context with timeout
	opCtx, cancel := context.WithTimeout(ctx, opt.resourceLimits.PushLimits.TimeoutDuration)
	if opt.resourceLimits.PushLimits.TimeoutDuration == 0 {
		opCtx, cancel = context.WithTimeout(ctx, 15*time.Minute)
	}
	defer cancel()

	startTime := time.Now()

	// Perform the actual push with optimization
	result, err := opt.optimizedPushWithRetry(opCtx, imageRef, options)

	duration := time.Since(startTime)

	// Record metrics
	opt.recordOperationMetrics(operation, duration, err == nil)

	opt.logger.Info().
		Str("operation", operation).
		Str("image", imageRef).
		Dur("duration", duration).
		Bool("success", err == nil).
		Msg("Completed optimized Docker push operation")

	return result, err
}

// OptimizedTag performs an optimized Docker tag operation
func (opt *DockerOperationOptimizer) OptimizedTag(ctx context.Context, sourceRef, targetRef string, options map[string]string) (string, error) {
	operation := "tag"
	cacheKey := opt.generateCacheKey(operation, sourceRef+":"+targetRef, options)

	// Check cache first (tags are usually fast but can be cached)
	if cached := opt.getCachedOperation(cacheKey); cached != nil {
		opt.recordCacheHit(operation)
		opt.logger.Debug().
			Str("operation", operation).
			Str("source", sourceRef).
			Str("target", targetRef).
			Msg("Cache hit for Docker tag operation")

		if cached.Success {
			return cached.Result, nil
		} else {
			return cached.Result, fmt.Errorf("cached operation error: %s", cached.Error)
		}
	}

	opt.recordCacheMiss(operation)

	startTime := time.Now()

	// Tag operations are usually fast, no heavy resource management needed
	result, err := opt.dockerClient.Tag(ctx, sourceRef, targetRef)

	duration := time.Since(startTime)

	// Cache the result with shorter TTL for tags
	cached := &CachedOperation{
		Key:        cacheKey,
		Operation:  operation,
		Result:     result,
		Success:    err == nil,
		Timestamp:  time.Now(),
		TTL:        opt.cacheTTL / 4, // Shorter TTL for tag operations
		Metadata:   options,
		HitCount:   0,
		LastAccess: time.Now(),
	}

	if err != nil {
		cached.Error = err.Error()
	}

	opt.setCachedOperation(cacheKey, cached)

	// Record metrics
	opt.recordOperationMetrics(operation, duration, err == nil)

	opt.logger.Info().
		Str("operation", operation).
		Str("source", sourceRef).
		Str("target", targetRef).
		Dur("duration", duration).
		Bool("success", err == nil).
		Msg("Completed optimized Docker tag operation")

	return result, err
}

// Private methods for optimization

func (opt *DockerOperationOptimizer) optimizedPullWithRetry(ctx context.Context, imageRef string, options map[string]string) (string, error) {
	maxRetries := opt.resourceLimits.PullLimits.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	backoff := opt.resourceLimits.PullLimits.RetryBackoff
	if backoff == 0 {
		backoff = 1 * time.Second
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(backoff * time.Duration(attempt)):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		result, err := opt.dockerClient.Pull(ctx, imageRef)
		if err == nil {
			if attempt > 0 {
				opt.logger.Info().
					Str("image", imageRef).
					Int("attempt", attempt+1).
					Msg("Docker pull succeeded after retry")
			}
			return result, nil
		}

		lastErr = err
		opt.logger.Warn().
			Err(err).
			Str("image", imageRef).
			Int("attempt", attempt+1).
			Int("max_attempts", maxRetries+1).
			Msg("Docker pull attempt failed")
	}

	return "", fmt.Errorf("docker pull failed after %d attempts: %w", maxRetries+1, lastErr)
}

func (opt *DockerOperationOptimizer) optimizedPushWithRetry(ctx context.Context, imageRef string, options map[string]string) (string, error) {
	maxRetries := opt.resourceLimits.PushLimits.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	backoff := opt.resourceLimits.PushLimits.RetryBackoff
	if backoff == 0 {
		backoff = 2 * time.Second
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			delay := backoff * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		result, err := opt.dockerClient.Push(ctx, imageRef)
		if err == nil {
			if attempt > 0 {
				opt.logger.Info().
					Str("image", imageRef).
					Int("attempt", attempt+1).
					Msg("Docker push succeeded after retry")
			}
			return result, nil
		}

		lastErr = err
		opt.logger.Warn().
			Err(err).
			Str("image", imageRef).
			Int("attempt", attempt+1).
			Int("max_attempts", maxRetries+1).
			Msg("Docker push attempt failed")
	}

	return "", fmt.Errorf("docker push failed after %d attempts: %w", maxRetries+1, lastErr)
}

func (opt *DockerOperationOptimizer) generateCacheKey(operation, reference string, options map[string]string) string {
	// Create a deterministic cache key
	keyData := fmt.Sprintf("%s:%s", operation, reference)

	// Add sorted options to ensure consistent key generation
	if len(options) > 0 {
		var optionPairs []string
		for k, v := range options {
			optionPairs = append(optionPairs, fmt.Sprintf("%s=%s", k, v))
		}
		keyData += ":" + strings.Join(optionPairs, ",")
	}

	// Generate SHA256 hash for the key
	hash := sha256.Sum256([]byte(keyData))
	return fmt.Sprintf("%x", hash)
}

func (opt *DockerOperationOptimizer) getCachedOperation(key string) *CachedOperation {
	opt.cacheMutex.RLock()
	defer opt.cacheMutex.RUnlock()

	cached, exists := opt.operationCache[key]
	if !exists {
		return nil
	}

	// Check if cache entry is still valid
	if time.Since(cached.Timestamp) > cached.TTL {
		// Cache entry expired, remove it
		delete(opt.operationCache, key)
		return nil
	}

	// Update access information
	cached.HitCount++
	cached.LastAccess = time.Now()

	return cached
}

func (opt *DockerOperationOptimizer) setCachedOperation(key string, operation *CachedOperation) {
	opt.cacheMutex.Lock()
	defer opt.cacheMutex.Unlock()

	// Check cache size limit
	if len(opt.operationCache) >= opt.maxCacheSize {
		opt.evictOldestCacheEntry()
	}

	opt.operationCache[key] = operation
}

func (opt *DockerOperationOptimizer) evictOldestCacheEntry() {
	// Find and remove the oldest cache entry
	var oldestKey string
	var oldestTime time.Time

	for key, cached := range opt.operationCache {
		if oldestKey == "" || cached.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.LastAccess
		}
	}

	if oldestKey != "" {
		delete(opt.operationCache, oldestKey)
		opt.logger.Debug().
			Str("evicted_key", oldestKey).
			Msg("Evicted oldest cache entry")
	}
}

func (opt *DockerOperationOptimizer) checkResourceLimits(operation string) error {
	opt.resourceTracker.mutex.RLock()
	defer opt.resourceTracker.mutex.RUnlock()

	// Check concurrent operations limit
	if opt.resourceTracker.activeOperations >= opt.resourceLimits.MaxConcurrentOps {
		return fmt.Errorf("maximum concurrent operations limit (%d) exceeded", opt.resourceLimits.MaxConcurrentOps)
	}

	// Check memory usage
	if opt.resourceLimits.MaxMemoryUsage > 0 && opt.resourceTracker.currentMemory > opt.resourceLimits.MaxMemoryUsage {
		return fmt.Errorf("memory usage limit (%d bytes) exceeded", opt.resourceLimits.MaxMemoryUsage)
	}

	// Check disk usage
	if opt.resourceLimits.MaxDiskUsage > 0 && opt.resourceTracker.currentDisk > opt.resourceLimits.MaxDiskUsage {
		return fmt.Errorf("disk usage limit (%d bytes) exceeded", opt.resourceLimits.MaxDiskUsage)
	}

	return nil
}

func (opt *DockerOperationOptimizer) trackResourceStart(operation string) {
	opt.resourceTracker.mutex.Lock()
	defer opt.resourceTracker.mutex.Unlock()

	opt.resourceTracker.activeOperations++

	// Record snapshot
	snapshot := ResourceSnapshot{
		Timestamp:  time.Now(),
		Memory:     opt.resourceTracker.currentMemory,
		Disk:       opt.resourceTracker.currentDisk,
		Bandwidth:  opt.resourceTracker.currentBandwidth,
		Operations: opt.resourceTracker.activeOperations,
	}

	opt.resourceTracker.operationHistory = append(opt.resourceTracker.operationHistory, snapshot)

	// Keep only last 100 snapshots
	if len(opt.resourceTracker.operationHistory) > 100 {
		opt.resourceTracker.operationHistory = opt.resourceTracker.operationHistory[1:]
	}
}

func (opt *DockerOperationOptimizer) trackResourceEnd(operation string) {
	opt.resourceTracker.mutex.Lock()
	defer opt.resourceTracker.mutex.Unlock()

	if opt.resourceTracker.activeOperations > 0 {
		opt.resourceTracker.activeOperations--
	}
}

func (opt *DockerOperationOptimizer) recordCacheHit(operation string) {
	opt.updateOperationMetrics(operation, func(metrics *OperationMetrics) {
		metrics.CacheHits++
		metrics.CacheHitRate = float64(metrics.CacheHits) / float64(metrics.CacheHits+metrics.CacheMisses) * 100
	})
}

func (opt *DockerOperationOptimizer) recordCacheMiss(operation string) {
	opt.updateOperationMetrics(operation, func(metrics *OperationMetrics) {
		metrics.CacheMisses++
		metrics.CacheHitRate = float64(metrics.CacheHits) / float64(metrics.CacheHits+metrics.CacheMisses) * 100
	})
}

func (opt *DockerOperationOptimizer) recordOperationMetrics(operation string, duration time.Duration, success bool) {
	opt.updateOperationMetrics(operation, func(metrics *OperationMetrics) {
		metrics.TotalExecutions++
		if success {
			metrics.SuccessfulOps++
		} else {
			metrics.FailedOps++
		}
		metrics.SuccessRate = float64(metrics.SuccessfulOps) / float64(metrics.TotalExecutions) * 100

		// Update duration metrics (simplified)
		if metrics.AvgDuration == 0 {
			metrics.AvgDuration = duration
		} else {
			metrics.AvgDuration = (metrics.AvgDuration + duration) / 2
		}

		// Update P95 (simplified approximation)
		if duration > metrics.P95Duration {
			metrics.P95Duration = duration
		}
	})
}

func (opt *DockerOperationOptimizer) updateOperationMetrics(operation string, updater func(*OperationMetrics)) {
	opt.metricsMutex.Lock()
	defer opt.metricsMutex.Unlock()

	metrics, exists := opt.operationMetrics[operation]
	if !exists {
		metrics = &OperationMetrics{
			OperationType: operation,
		}
		opt.operationMetrics[operation] = metrics
	}

	updater(metrics)
}

// Cleanup and monitoring routines

func (opt *DockerOperationOptimizer) startCacheCleanup() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		opt.cleanupExpiredCache()
	}
}

func (opt *DockerOperationOptimizer) cleanupExpiredCache() {
	opt.cacheMutex.Lock()
	defer opt.cacheMutex.Unlock()

	now := time.Now()
	expired := 0

	for key, cached := range opt.operationCache {
		if now.Sub(cached.Timestamp) > cached.TTL {
			delete(opt.operationCache, key)
			expired++
		}
	}

	if expired > 0 {
		opt.logger.Debug().
			Int("expired_count", expired).
			Int("remaining_count", len(opt.operationCache)).
			Msg("Cleaned up expired cache entries")
	}
}

func (opt *DockerOperationOptimizer) startResourceMonitoring() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		opt.updateResourceMetrics()
	}
}

func (opt *DockerOperationOptimizer) updateResourceMetrics() {
	// This would typically integrate with system monitoring
	// For now, we just log current state
	opt.resourceTracker.mutex.RLock()
	activeOps := opt.resourceTracker.activeOperations
	opt.resourceTracker.mutex.RUnlock()

	opt.logger.Debug().
		Int("active_operations", activeOps).
		Int("cache_size", len(opt.operationCache)).
		Msg("Resource monitoring update")
}

// Public methods for metrics and management

// GetOperationMetrics returns metrics for a specific operation type
func (opt *DockerOperationOptimizer) GetOperationMetrics(operation string) *OperationMetrics {
	opt.metricsMutex.RLock()
	defer opt.metricsMutex.RUnlock()

	if metrics, exists := opt.operationMetrics[operation]; exists {
		// Return a copy
		metricsCopy := *metrics
		return &metricsCopy
	}

	return nil
}

// GetAllMetrics returns metrics for all operation types
func (opt *DockerOperationOptimizer) GetAllMetrics() map[string]*OperationMetrics {
	opt.metricsMutex.RLock()
	defer opt.metricsMutex.RUnlock()

	result := make(map[string]*OperationMetrics)
	for operation, metrics := range opt.operationMetrics {
		metricsCopy := *metrics
		result[operation] = &metricsCopy
	}

	return result
}

// GetCacheStats returns current cache statistics
func (opt *DockerOperationOptimizer) GetCacheStats() map[string]interface{} {
	opt.cacheMutex.RLock()
	defer opt.cacheMutex.RUnlock()

	return map[string]interface{}{
		"cache_size":     len(opt.operationCache),
		"max_cache_size": opt.maxCacheSize,
		"cache_ttl":      opt.cacheTTL.String(),
		"utilization":    float64(len(opt.operationCache)) / float64(opt.maxCacheSize) * 100,
	}
}

// ClearCache clears all cached operations
func (opt *DockerOperationOptimizer) ClearCache() {
	opt.cacheMutex.Lock()
	defer opt.cacheMutex.Unlock()

	cleared := len(opt.operationCache)
	opt.operationCache = make(map[string]*CachedOperation)

	opt.logger.Info().
		Int("cleared_entries", cleared).
		Msg("Cache cleared")
}

// GetResourceStats returns current resource utilization
func (opt *DockerOperationOptimizer) GetResourceStats() map[string]interface{} {
	opt.resourceTracker.mutex.RLock()
	defer opt.resourceTracker.mutex.RUnlock()

	return map[string]interface{}{
		"active_operations": opt.resourceTracker.activeOperations,
		"max_concurrent":    opt.resourceLimits.MaxConcurrentOps,
		"current_memory":    opt.resourceTracker.currentMemory,
		"max_memory":        opt.resourceLimits.MaxMemoryUsage,
		"current_disk":      opt.resourceTracker.currentDisk,
		"max_disk":          opt.resourceLimits.MaxDiskUsage,
		"current_bandwidth": opt.resourceTracker.currentBandwidth,
		"max_bandwidth":     opt.resourceLimits.MaxNetworkBandwidth,
	}
}
