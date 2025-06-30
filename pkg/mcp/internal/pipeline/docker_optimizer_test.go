package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/runner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerOperationOptimizer_NewDockerOperationOptimizer(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{
		EnableCaching:      true,
		CacheTTL:           30 * time.Minute,
		MaxCacheSize:       500,
		MaxConcurrent:      5,
		EnableLayerCache:   true,
		EnableRegistryPool: true,
		ResourceLimits: ResourceLimits{
			MaxConcurrentOps: 3,
			MaxOperationTime: 5 * time.Minute,
			PullLimits: OperationLimits{
				MaxConcurrent:   2,
				MaxDuration:     3 * time.Minute,
				MaxRetries:      2,
				RetryBackoff:    2 * time.Second,
				TimeoutDuration: 5 * time.Minute,
			},
			PushLimits: OperationLimits{
				MaxConcurrent:   1,
				MaxDuration:     5 * time.Minute,
				MaxRetries:      3,
				RetryBackoff:    3 * time.Second,
				TimeoutDuration: 10 * time.Minute,
			},
		},
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	assert.NotNil(t, optimizer)
	assert.Equal(t, config.CacheTTL, optimizer.cacheTTL)
	assert.Equal(t, config.MaxCacheSize, optimizer.maxCacheSize)
	assert.Equal(t, config.MaxConcurrent, optimizer.maxConcurrent)
	assert.NotNil(t, optimizer.operationCache)
	assert.NotNil(t, optimizer.operationMetrics)
	assert.NotNil(t, optimizer.resourceTracker)
}

func TestDockerOperationOptimizer_CacheOperations(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create fake docker client that returns predictable results
	fakeRunner := &runner.FakeCommandRunner{
		Output: "Successfully pulled nginx:latest",
		ErrStr: "",
	}
	dockerClient := docker.NewDockerCmdRunner(fakeRunner)

	config := OptimizationConfig{
		EnableCaching: true,
		CacheTTL:      5 * time.Minute,
		MaxCacheSize:  100,
		MaxConcurrent: 3,
		ResourceLimits: ResourceLimits{
			MaxConcurrentOps: 5,
		},
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)
	ctx := context.Background()

	// First call should miss cache and execute
	result1, err1 := optimizer.OptimizedPull(ctx, "nginx:latest", nil)
	require.NoError(t, err1)
	assert.Contains(t, result1, "Successfully pulled")

	// Check that metrics show cache miss
	metrics := optimizer.GetOperationMetrics("pull")
	require.NotNil(t, metrics)
	assert.Equal(t, 1, metrics.CacheMisses)
	assert.Equal(t, 0, metrics.CacheHits)
	assert.Equal(t, 1, metrics.TotalExecutions)

	// Second call should hit cache
	result2, err2 := optimizer.OptimizedPull(ctx, "nginx:latest", nil)
	require.NoError(t, err2)
	assert.Equal(t, result1, result2)

	// Check that metrics show cache hit
	metrics = optimizer.GetOperationMetrics("pull")
	assert.Equal(t, 1, metrics.CacheMisses)
	assert.Equal(t, 1, metrics.CacheHits)
	assert.Equal(t, 50.0, metrics.CacheHitRate)
}

func TestDockerOperationOptimizer_CacheKeyGeneration(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{}
	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Test that different operations generate different keys
	key1 := optimizer.generateCacheKey("pull", "nginx:latest", nil)
	key2 := optimizer.generateCacheKey("push", "nginx:latest", nil)
	key3 := optimizer.generateCacheKey("pull", "alpine:latest", nil)

	assert.NotEqual(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.NotEqual(t, key2, key3)

	// Test that same parameters generate same key
	key4 := optimizer.generateCacheKey("pull", "nginx:latest", nil)
	assert.Equal(t, key1, key4)

	// Test that options affect the key
	options := map[string]string{"registry": "docker.io"}
	key5 := optimizer.generateCacheKey("pull", "nginx:latest", options)
	assert.NotEqual(t, key1, key5)

	// Test that same options generate same key
	key6 := optimizer.generateCacheKey("pull", "nginx:latest", options)
	assert.Equal(t, key5, key6)
}

func TestDockerOperationOptimizer_ResourceLimits(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{
		ResourceLimits: ResourceLimits{
			MaxConcurrentOps: 1,    // Very low limit for testing
			MaxMemoryUsage:   1024, // 1KB limit
			MaxDiskUsage:     2048, // 2KB limit
		},
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Test concurrent operations limit
	err := optimizer.checkResourceLimits("pull")
	assert.NoError(t, err)

	// Simulate active operation
	optimizer.trackResourceStart("pull")

	// Should now fail due to concurrent limit
	err = optimizer.checkResourceLimits("pull")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum concurrent operations limit")

	// Clean up
	optimizer.trackResourceEnd("pull")

	// Should work again
	err = optimizer.checkResourceLimits("pull")
	assert.NoError(t, err)
}

func TestDockerOperationOptimizer_CacheEviction(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{Output: "success"})

	config := OptimizationConfig{
		EnableCaching: true,
		MaxCacheSize:  2, // Very small cache for testing eviction
		CacheTTL:      1 * time.Hour,
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Add first cache entry
	cached1 := &CachedOperation{
		Key:        "key1",
		Operation:  "pull",
		Result:     "result1",
		Success:    true,
		Timestamp:  time.Now(),
		TTL:        1 * time.Hour,
		LastAccess: time.Now().Add(-1 * time.Hour), // Make it older
	}
	optimizer.setCachedOperation("key1", cached1)

	// Add second cache entry
	cached2 := &CachedOperation{
		Key:        "key2",
		Operation:  "pull",
		Result:     "result2",
		Success:    true,
		Timestamp:  time.Now(),
		TTL:        1 * time.Hour,
		LastAccess: time.Now(),
	}
	optimizer.setCachedOperation("key2", cached2)

	// Cache should be at capacity
	stats := optimizer.GetCacheStats()
	assert.Equal(t, 2.0, stats["cache_size"])

	// Add third entry, should evict oldest
	cached3 := &CachedOperation{
		Key:        "key3",
		Operation:  "pull",
		Result:     "result3",
		Success:    true,
		Timestamp:  time.Now(),
		TTL:        1 * time.Hour,
		LastAccess: time.Now(),
	}
	optimizer.setCachedOperation("key3", cached3)

	// Cache should still be at capacity, but oldest entry should be gone
	stats = optimizer.GetCacheStats()
	assert.Equal(t, 2.0, stats["cache_size"])

	// key1 should be evicted, key2 and key3 should remain
	assert.Nil(t, optimizer.getCachedOperation("key1"))
	assert.NotNil(t, optimizer.getCachedOperation("key2"))
	assert.NotNil(t, optimizer.getCachedOperation("key3"))
}

func TestDockerOperationOptimizer_CacheExpiration(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{
		EnableCaching: true,
		CacheTTL:      100 * time.Millisecond, // Very short TTL for testing
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Add cache entry
	cached := &CachedOperation{
		Key:        "test_key",
		Operation:  "pull",
		Result:     "test_result",
		Success:    true,
		Timestamp:  time.Now(),
		TTL:        100 * time.Millisecond,
		LastAccess: time.Now(),
	}
	optimizer.setCachedOperation("test_key", cached)

	// Should be accessible immediately
	result := optimizer.getCachedOperation("test_key")
	assert.NotNil(t, result)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired and return nil
	result = optimizer.getCachedOperation("test_key")
	assert.Nil(t, result)
}

func TestDockerOperationOptimizer_TagOperations(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	fakeRunner := &runner.FakeCommandRunner{
		Output: "Tag operation successful",
		ErrStr: "",
	}
	dockerClient := docker.NewDockerCmdRunner(fakeRunner)

	config := OptimizationConfig{
		EnableCaching: true,
		CacheTTL:      5 * time.Minute,
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)
	ctx := context.Background()

	// Test tag operation
	result, err := optimizer.OptimizedTag(ctx, "nginx:latest", "nginx:v1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, result, "Tag operation successful")

	// Check metrics
	metrics := optimizer.GetOperationMetrics("tag")
	require.NotNil(t, metrics)
	assert.Equal(t, 1, metrics.TotalExecutions)
	assert.Equal(t, 1, metrics.SuccessfulOps)
	assert.Equal(t, 0, metrics.FailedOps)
	assert.Equal(t, 100.0, metrics.SuccessRate)

	// Second call should hit cache
	result2, err2 := optimizer.OptimizedTag(ctx, "nginx:latest", "nginx:v1.0", nil)
	require.NoError(t, err2)
	assert.Equal(t, result, result2)

	// Check cache hit
	metrics = optimizer.GetOperationMetrics("tag")
	assert.Equal(t, 1, metrics.CacheHits)
	assert.Equal(t, 50.0, metrics.CacheHitRate)
}

func TestDockerOperationOptimizer_MetricsTracking(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{Output: "success"})

	config := OptimizationConfig{}
	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Record some operation metrics
	optimizer.recordOperationMetrics("pull", 100*time.Millisecond, true)
	optimizer.recordOperationMetrics("pull", 200*time.Millisecond, true)
	optimizer.recordOperationMetrics("pull", 150*time.Millisecond, false)

	metrics := optimizer.GetOperationMetrics("pull")
	require.NotNil(t, metrics)

	assert.Equal(t, "pull", metrics.OperationType)
	assert.Equal(t, 3, metrics.TotalExecutions)
	assert.Equal(t, 2, metrics.SuccessfulOps)
	assert.Equal(t, 1, metrics.FailedOps)
	assert.InDelta(t, 66.67, metrics.SuccessRate, 0.1)
	assert.True(t, metrics.AvgDuration > 0)
	assert.True(t, metrics.P95Duration > 0)
}

func TestDockerOperationOptimizer_AllMetrics(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{Output: "success"})

	config := OptimizationConfig{}
	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Record metrics for different operations
	optimizer.recordOperationMetrics("pull", 100*time.Millisecond, true)
	optimizer.recordOperationMetrics("push", 200*time.Millisecond, true)
	optimizer.recordOperationMetrics("tag", 50*time.Millisecond, true)

	allMetrics := optimizer.GetAllMetrics()

	assert.Len(t, allMetrics, 3)
	assert.Contains(t, allMetrics, "pull")
	assert.Contains(t, allMetrics, "push")
	assert.Contains(t, allMetrics, "tag")

	assert.Equal(t, 1, allMetrics["pull"].TotalExecutions)
	assert.Equal(t, 1, allMetrics["push"].TotalExecutions)
	assert.Equal(t, 1, allMetrics["tag"].TotalExecutions)
}

func TestDockerOperationOptimizer_CacheStats(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{
		MaxCacheSize: 100,
		CacheTTL:     1 * time.Hour,
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Add some cache entries
	for i := 0; i < 10; i++ {
		cached := &CachedOperation{
			Key:        fmt.Sprintf("key%d", i),
			Operation:  "pull",
			Result:     "result",
			Success:    true,
			Timestamp:  time.Now(),
			TTL:        1 * time.Hour,
			LastAccess: time.Now(),
		}
		optimizer.setCachedOperation(cached.Key, cached)
	}

	stats := optimizer.GetCacheStats()

	assert.Equal(t, 10.0, stats["cache_size"])
	assert.Equal(t, 100, stats["max_cache_size"])
	assert.Equal(t, "1h0m0s", stats["cache_ttl"])
	assert.Equal(t, 10.0, stats["utilization"])
}

func TestDockerOperationOptimizer_ResourceStats(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{
		ResourceLimits: ResourceLimits{
			MaxConcurrentOps:    5,
			MaxMemoryUsage:      1024 * 1024,      // 1MB
			MaxDiskUsage:        10 * 1024 * 1024, // 10MB
			MaxNetworkBandwidth: 1024 * 1024,      // 1MB/s
		},
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	stats := optimizer.GetResourceStats()

	assert.Equal(t, 0, stats["active_operations"])
	assert.Equal(t, 5, stats["max_concurrent"])
	assert.Equal(t, int64(0), stats["current_memory"])
	assert.Equal(t, int64(1024*1024), stats["max_memory"])
	assert.Equal(t, int64(0), stats["current_disk"])
	assert.Equal(t, int64(10*1024*1024), stats["max_disk"])
}

func TestDockerOperationOptimizer_ClearCache(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{}
	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	// Add cache entries
	for i := 0; i < 5; i++ {
		cached := &CachedOperation{
			Key:        fmt.Sprintf("key%d", i),
			Operation:  "pull",
			Result:     "result",
			Success:    true,
			Timestamp:  time.Now(),
			TTL:        1 * time.Hour,
			LastAccess: time.Now(),
		}
		optimizer.setCachedOperation(cached.Key, cached)
	}

	// Verify cache has entries
	stats := optimizer.GetCacheStats()
	assert.Equal(t, 5.0, stats["cache_size"])

	// Clear cache
	optimizer.ClearCache()

	// Verify cache is empty
	stats = optimizer.GetCacheStats()
	assert.Equal(t, 0.0, stats["cache_size"])
}

// Benchmark tests
func BenchmarkDockerOperationOptimizer_CacheKeyGeneration(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{})

	config := OptimizationConfig{}
	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	options := map[string]string{
		"registry": "docker.io",
		"tag":      "latest",
		"platform": "linux/amd64",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = optimizer.generateCacheKey("pull", "nginx:latest", options)
	}
}

func BenchmarkDockerOperationOptimizer_CacheOperations(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))
	dockerClient := docker.NewDockerCmdRunner(&runner.FakeCommandRunner{Output: "success"})

	config := OptimizationConfig{
		EnableCaching: true,
		MaxCacheSize:  1000,
		CacheTTL:      1 * time.Hour,
	}

	optimizer := NewDockerOperationOptimizer(dockerClient, config, logger)

	cached := &CachedOperation{
		Key:        "benchmark_key",
		Operation:  "pull",
		Result:     "benchmark_result",
		Success:    true,
		Timestamp:  time.Now(),
		TTL:        1 * time.Hour,
		LastAccess: time.Now(),
	}
	optimizer.setCachedOperation("benchmark_key", cached)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = optimizer.getCachedOperation("benchmark_key")
	}
}
