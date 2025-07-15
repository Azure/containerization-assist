package caching

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_BasicOperations(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(100, 1*time.Minute)
	defer cache.Stop()

	// Test Set and Get
	cache.Set(ctx, "key1", "value1", 5*time.Minute)

	value, found := cache.Get(ctx, "key1")
	assert.True(t, found)
	assert.Equal(t, "value1", value)

	// Test Get non-existent key
	_, found = cache.Get(ctx, "nonexistent")
	assert.False(t, found)

	// Test Delete
	cache.Delete(ctx, "key1")
	_, found = cache.Get(ctx, "key1")
	assert.False(t, found)

	// Test Clear
	cache.Set(ctx, "key2", "value2", 5*time.Minute)
	cache.Set(ctx, "key3", "value3", 5*time.Minute)
	cache.Clear(ctx)

	_, found = cache.Get(ctx, "key2")
	assert.False(t, found)
	_, found = cache.Get(ctx, "key3")
	assert.False(t, found)
}

func TestMemoryCache_Expiration(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(100, 100*time.Millisecond)
	defer cache.Stop()

	// Set with short TTL
	cache.Set(ctx, "key1", "value1", 200*time.Millisecond)

	// Should exist initially
	_, found := cache.Get(ctx, "key1")
	assert.True(t, found)

	// Wait for expiration
	time.Sleep(300 * time.Millisecond)

	// Should be expired
	_, found = cache.Get(ctx, "key1")
	assert.False(t, found)
}

func TestMemoryCache_LRUEviction(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(3, 1*time.Minute)
	defer cache.Stop()

	// Fill cache to capacity
	cache.Set(ctx, "key1", "value1", 5*time.Minute)
	time.Sleep(10 * time.Millisecond)
	cache.Set(ctx, "key2", "value2", 5*time.Minute)
	time.Sleep(10 * time.Millisecond)
	cache.Set(ctx, "key3", "value3", 5*time.Minute)

	// Access key1 to make it recently used
	cache.Get(ctx, "key1")

	// Add new key, should evict key2 (least recently used)
	cache.Set(ctx, "key4", "value4", 5*time.Minute)

	// key1 should still exist (recently accessed)
	_, found := cache.Get(ctx, "key1")
	assert.True(t, found)

	// key2 should be evicted
	_, found = cache.Get(ctx, "key2")
	assert.False(t, found)

	// key3 and key4 should exist
	_, found = cache.Get(ctx, "key3")
	assert.True(t, found)
	_, found = cache.Get(ctx, "key4")
	assert.True(t, found)
}

func TestMemoryCache_Stats(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(100, 1*time.Minute)
	defer cache.Stop()

	// Initial stats
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, int64(0), stats.Sets)
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 100, stats.MaxSize)

	// Set some values
	cache.Set(ctx, "key1", "value1", 5*time.Minute)
	cache.Set(ctx, "key2", "value2", 5*time.Minute)

	// Get operations
	cache.Get(ctx, "key1") // Hit
	cache.Get(ctx, "key3") // Miss

	// Delete operation
	cache.Delete(ctx, "key2")

	// Check stats
	stats = cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(2), stats.Sets)
	assert.Equal(t, int64(1), stats.Deletes)
	assert.Equal(t, 1, stats.Size)
}

func TestLayeredCache(t *testing.T) {
	ctx := context.Background()

	// Create two cache layers
	l1 := NewMemoryCache(10, 1*time.Minute)
	l2 := NewMemoryCache(100, 1*time.Minute)
	defer l1.Stop()
	defer l2.Stop()

	layered := NewLayeredCache(l1, l2)

	// Set value in L2 only
	l2.Set(ctx, "key1", "value1", 5*time.Minute)

	// Get should find in L2 and promote to L1
	value, found := layered.Get(ctx, "key1")
	assert.True(t, found)
	assert.Equal(t, "value1", value)

	// Now should be in L1 as well
	value, found = l1.Get(ctx, "key1")
	assert.True(t, found)
	assert.Equal(t, "value1", value)

	// Set through layered cache
	layered.Set(ctx, "key2", "value2", 5*time.Minute)

	// Should be in both layers
	_, found = l1.Get(ctx, "key2")
	assert.True(t, found)
	_, found = l2.Get(ctx, "key2")
	assert.True(t, found)

	// Delete through layered cache
	layered.Delete(ctx, "key2")

	// Should be gone from both layers
	_, found = l1.Get(ctx, "key2")
	assert.False(t, found)
	_, found = l2.Get(ctx, "key2")
	assert.False(t, found)
}

func TestCacheKey(t *testing.T) {
	key := CacheKey("namespace", "type", "id")
	assert.Equal(t, "namespace:type:id", key)

	key = CacheKey("single")
	assert.Equal(t, "single", key)

	key = CacheKey()
	assert.Equal(t, "", key)
}

func TestSerializableCache(t *testing.T) {
	ctx := context.Background()
	memCache := NewMemoryCache(100, 1*time.Minute)
	defer memCache.Stop()

	cache := NewSerializableCache(memCache)

	// Test with struct
	type TestStruct struct {
		Name  string
		Value int
	}

	original := TestStruct{Name: "test", Value: 42}

	// Set value
	err := cache.Set(ctx, "key1", original, 5*time.Minute)
	require.NoError(t, err)

	// Get value
	var retrieved TestStruct
	err = cache.Get(ctx, "key1", &retrieved)
	require.NoError(t, err)
	assert.Equal(t, original, retrieved)

	// Test cache miss
	var missing TestStruct
	err = cache.Get(ctx, "nonexistent", &missing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")
}

func TestMemoryCache_Concurrency(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(1000, 1*time.Minute)
	defer cache.Stop()

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := CacheKey("worker", string(rune(id)), string(rune(j)))
				cache.Set(ctx, key, id*100+j, 5*time.Minute)
			}
			done <- true
		}(i)
	}

	// Wait for writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := CacheKey("worker", string(rune(id)), string(rune(j)))
				value, found := cache.Get(ctx, key)
				if found {
					expected := id*100 + j
					assert.Equal(t, expected, value)
				}
			}
			done <- true
		}(i)
	}

	// Wait for reads
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := cache.Stats()
	assert.Greater(t, stats.Hits, int64(0))
	assert.Greater(t, stats.Sets, int64(0))
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	ctx := context.Background()
	cache := NewMemoryCache(10000, 1*time.Minute)
	defer cache.Stop()

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := CacheKey("bench", string(rune(i)))
		cache.Set(ctx, key, i, 5*time.Minute)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := CacheKey("bench", string(rune(i%1000)))
			cache.Get(ctx, key)
			i++
		}
	})
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	ctx := context.Background()
	cache := NewMemoryCache(10000, 1*time.Minute)
	defer cache.Stop()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := CacheKey("bench", string(rune(i)))
			cache.Set(ctx, key, i, 5*time.Minute)
			i++
		}
	})
}
