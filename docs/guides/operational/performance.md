# Performance Optimization Guide

Container Kit maintains strict performance standards with a target of <300μs P95 response time. This guide covers performance optimization techniques, monitoring, and best practices.

## Performance Targets

### Response Time Requirements
- **Tool Execution**: <300μs P95 latency
- **File Operations**: <100μs P95 latency
- **Session Operations**: <50μs P95 latency
- **Database Operations**: <25μs P95 latency

### Throughput Requirements
- **Concurrent Sessions**: 1000+ simultaneous sessions
- **Tool Execution**: 10,000+ operations per second
- **File Operations**: 50,000+ operations per second

## Performance Monitoring

### Benchmarking Commands
```bash
# Run performance benchmarks
make bench

# Set performance baseline
make bench-baseline

# Compare against baseline
make bench-compare

# Generate performance report
make bench-report
```

### Benchmark Output
```
BenchmarkAnalyzeRepository-8     5000    287.4 μs/op    1024 B/op    15 allocs/op
BenchmarkBuildImage-8           1000    1.2 ms/op      2048 B/op    25 allocs/op
BenchmarkFileAccess-8          10000    89.3 μs/op      512 B/op     8 allocs/op
```

## Performance Architecture

### Service Container Optimization
```go
// Lazy initialization reduces startup time
type serviceContainer struct {
    fileAccessOnce sync.Once
    fileAccess     FileAccessService
    
    sessionStoreOnce sync.Once
    sessionStore     SessionStore
}

func (c *serviceContainer) FileAccessService() FileAccessService {
    c.fileAccessOnce.Do(func() {
        c.fileAccess = &optimizedFileAccessService{
            cache: newLRUCache(1000),
            pool:  newWorkerPool(10),
        }
    })
    return c.fileAccess
}
```

### Connection Pooling
```go
type OptimizedDockerClient struct {
    client *client.Client
    pool   *connectionPool
}

func (c *OptimizedDockerClient) Build(ctx context.Context, req *BuildRequest) (*BuildResponse, error) {
    // Reuse connections from pool
    conn := c.pool.Get()
    defer c.pool.Put(conn)
    
    // Execute build operation
    return c.executeBuild(ctx, conn, req)
}
```

## Optimization Strategies

### 1. Caching Strategy
```go
type CachedAnalysisService struct {
    cache *lru.Cache
    store AnalysisStore
}

func (s *CachedAnalysisService) AnalyzeRepository(ctx context.Context, path string) (*AnalysisResult, error) {
    // Check cache first
    if result, ok := s.cache.Get(path); ok {
        return result.(*AnalysisResult), nil
    }
    
    // Perform analysis
    result, err := s.store.Analyze(ctx, path)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    s.cache.Set(path, result, 5*time.Minute)
    return result, nil
}
```

### 2. Memory Pool Management
```go
type BufferPool struct {
    pool sync.Pool
}

func NewBufferPool() *BufferPool {
    return &BufferPool{
        pool: sync.Pool{
            New: func() interface{} {
                return make([]byte, 4096)
            },
        },
    }
}

func (p *BufferPool) Get() []byte {
    return p.pool.Get().([]byte)
}

func (p *BufferPool) Put(b []byte) {
    p.pool.Put(b[:0])
}
```

### 3. Goroutine Pool
```go
type WorkerPool struct {
    workers chan struct{}
    queue   chan func()
}

func NewWorkerPool(size int) *WorkerPool {
    wp := &WorkerPool{
        workers: make(chan struct{}, size),
        queue:   make(chan func(), size*2),
    }
    
    for i := 0; i < size; i++ {
        go wp.worker()
    }
    
    return wp
}

func (wp *WorkerPool) Submit(fn func()) {
    wp.queue <- fn
}

func (wp *WorkerPool) worker() {
    for fn := range wp.queue {
        fn()
    }
}
```

## FileAccessService Optimization

### Efficient File Operations
```go
type OptimizedFileAccessService struct {
    cache    *lru.Cache
    pool     *WorkerPool
    sessions map[string]*SessionContext
    mutex    sync.RWMutex
}

func (s *OptimizedFileAccessService) ReadFile(ctx context.Context, sessionID, path string) (string, error) {
    // Fast path: check cache
    cacheKey := fmt.Sprintf("%s:%s", sessionID, path)
    if content, ok := s.cache.Get(cacheKey); ok {
        return content.(string), nil
    }
    
    // Optimized file reading
    content, err := s.readFileOptimized(ctx, sessionID, path)
    if err != nil {
        return "", err
    }
    
    // Cache for future reads
    s.cache.Set(cacheKey, content, 2*time.Minute)
    return content, nil
}

func (s *OptimizedFileAccessService) readFileOptimized(ctx context.Context, sessionID, path string) (string, error) {
    // Pre-allocate buffer based on file size
    info, err := os.Stat(path)
    if err != nil {
        return "", err
    }
    
    // Use buffer pool for large files
    if info.Size() > 1024 {
        buffer := s.bufferPool.Get()
        defer s.bufferPool.Put(buffer)
        
        return s.readWithBuffer(path, buffer)
    }
    
    // Direct read for small files
    data, err := os.ReadFile(path)
    return string(data), err
}
```

## Database Optimization

### BoltDB Performance Tuning
```go
func optimizeBoltDB(db *bolt.DB) error {
    // Increase page size for better performance
    db.MaxBatchSize = 1000
    db.MaxBatchDelay = 10 * time.Millisecond
    
    // Use batch operations for multiple writes
    return db.Batch(func(tx *bolt.Tx) error {
        // Batch multiple operations
        return nil
    })
}

// Optimized session operations
func (s *SessionStore) BatchUpdateSessions(sessions []*Session) error {
    return s.db.Batch(func(tx *bolt.Tx) error {
        bucket := tx.Bucket([]byte("sessions"))
        for _, session := range sessions {
            data, err := json.Marshal(session)
            if err != nil {
                return err
            }
            if err := bucket.Put([]byte(session.ID), data); err != nil {
                return err
            }
        }
        return nil
    })
}
```

## Memory Management

### Memory Profiling
```go
func enableMemoryProfiling() {
    // Enable memory profiling
    runtime.SetMemProfileRate(1)
    
    // Periodic memory stats
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            log.Printf("Memory: Alloc=%d KB, TotalAlloc=%d KB, Sys=%d KB, GC=%d",
                m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC)
        }
    }()
}
```

### Garbage Collection Optimization
```go
func optimizeGC() {
    // Tune GC target percentage
    debug.SetGCPercent(100)
    
    // Force GC at strategic points
    runtime.GC()
    
    // Monitor GC performance
    var stats debug.GCStats
    debug.ReadGCStats(&stats)
    
    log.Printf("GC: NumGC=%d, PauseTotal=%v, LastGC=%v",
        stats.NumGC, stats.PauseTotal, stats.LastGC)
}
```

## Concurrent Processing

### Parallel Analysis
```go
func (s *AnalysisService) AnalyzeRepositoryParallel(ctx context.Context, path string) (*AnalysisResult, error) {
    var wg sync.WaitGroup
    results := make(chan *PartialResult, 4)
    
    // Analyze components in parallel
    wg.Add(4)
    go s.analyzeLanguage(ctx, path, results, &wg)
    go s.analyzeDependencies(ctx, path, results, &wg)
    go s.analyzeFramework(ctx, path, results, &wg)
    go s.analyzeConfig(ctx, path, results, &wg)
    
    // Wait for completion
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Combine results
    return s.combineResults(results), nil
}
```

### Non-blocking Operations
```go
func (s *BuildService) BuildImageAsync(ctx context.Context, req *BuildRequest) (*BuildResponse, error) {
    // Start build in background
    buildChan := make(chan *BuildResult, 1)
    go func() {
        result, err := s.performBuild(ctx, req)
        buildChan <- &BuildResult{Result: result, Error: err}
    }()
    
    // Return immediately with build ID
    buildID := generateBuildID()
    s.trackBuild(buildID, buildChan)
    
    return &BuildResponse{
        BuildID: buildID,
        Status:  "started",
    }, nil
}
```

## Monitoring and Metrics

### Performance Metrics Collection
```go
type PerformanceMetrics struct {
    requestDuration prometheus.HistogramVec
    requestCount    prometheus.CounterVec
    errorCount      prometheus.CounterVec
    memoryUsage     prometheus.GaugeVec
}

func NewPerformanceMetrics() *PerformanceMetrics {
    return &PerformanceMetrics{
        requestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "container_kit_request_duration_seconds",
                Help:    "Request duration in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
            },
            []string{"tool", "status"},
        ),
        requestCount: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "container_kit_requests_total",
                Help: "Total number of requests",
            },
            []string{"tool", "status"},
        ),
    }
}
```

### Real-time Performance Monitoring
```go
func (m *PerformanceMetrics) RecordRequest(tool string, duration time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "error"
        m.errorCount.WithLabelValues(tool).Inc()
    }
    
    m.requestDuration.WithLabelValues(tool, status).Observe(duration.Seconds())
    m.requestCount.WithLabelValues(tool, status).Inc()
}
```

## Performance Testing

### Load Testing
```go
func BenchmarkConcurrentAnalysis(b *testing.B) {
    service := setupAnalysisService()
    
    b.SetParallelism(100) // 100 concurrent goroutines
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            ctx := context.Background()
            _, err := service.AnalyzeRepository(ctx, "test-repo")
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

### Memory Benchmarks
```go
func BenchmarkMemoryUsage(b *testing.B) {
    service := setupAnalysisService()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        result, err := service.AnalyzeRepository(context.Background(), "test-repo")
        if err != nil {
            b.Fatal(err)
        }
        _ = result
    }
}
```

## Performance Best Practices

### 1. Efficient Data Structures
- Use `sync.Map` for concurrent access
- Implement object pooling for frequently used objects
- Use channels for goroutine communication
- Prefer slices over maps for small collections

### 2. I/O Optimization
- Use buffered I/O for file operations
- Implement read-ahead caching
- Use memory-mapped files for large datasets
- Batch database operations

### 3. CPU Optimization
- Minimize allocations in hot paths
- Use efficient algorithms and data structures
- Implement CPU-bound operations in parallel
- Profile and optimize critical sections

### 4. Memory Optimization
- Reuse objects through pooling
- Minimize string concatenation
- Use byte slices instead of strings where possible
- Implement proper garbage collection tuning

## Troubleshooting Performance Issues

### Common Performance Problems

1. **High Latency**
   - Check for blocking operations
   - Optimize database queries
   - Implement proper caching

2. **Memory Leaks**
   - Monitor goroutine count
   - Check for unclosed resources
   - Use memory profiling tools

3. **CPU Bottlenecks**
   - Profile CPU usage
   - Optimize hot code paths
   - Implement parallel processing

### Performance Debugging
```bash
# CPU profiling
go tool pprof http://localhost:6060/debug/pprof/profile

# Memory profiling
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine profiling
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Custom benchmark profiling
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
```

## Related Documentation

- [Testing Guide](testing.md)
- [Monitoring Guide](monitoring.md)
- [Architecture Overview](../../architecture/overview.md)
- [Service Container](../../architecture/service-container.md)

Container Kit's performance optimization strategy ensures consistent high performance while maintaining code quality and maintainability.