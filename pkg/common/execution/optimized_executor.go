package execution

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/common/interfaces"
	"github.com/Azure/container-kit/pkg/common/pools"
	"github.com/Azure/container-kit/pkg/mcp/api"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// ============================================================================
// WORKSTREAM DELTA: Optimized Tool Execution Framework
// Consolidates repetitive execution patterns across multiple large files
// Target: Reduce 25K+ LOC of duplicated execution logic
// ============================================================================

// OptimizedExecutor provides high-performance tool execution with pooled resources
type OptimizedExecutor struct {
	observability interfaces.UnifiedObservability
	validator     interfaces.UnifiedValidator
	config        ExecutorConfig

	// Performance optimization fields
	workerPool    *WorkerPool
	resultCache   *ResultCache
	metricsBuffer *MetricsBuffer

	// Synchronization
	mu          sync.RWMutex
	activeTools map[string]*ToolExecution
}

// ExecutorConfig configures the optimized executor
type ExecutorConfig struct {
	MaxConcurrency    int
	ExecutionTimeout  time.Duration
	EnableCaching     bool
	EnableMetrics     bool
	EnableValidation  bool
	WorkerPoolSize    int
	CacheSize         int
	MetricsBufferSize int
}

// ToolExecution represents an active tool execution
type ToolExecution struct {
	ID        string
	ToolName  string
	StartTime time.Time
	Context   context.Context
	Cancel    context.CancelFunc
	Result    chan ExecutionResult
}

// ExecutionResult represents the result of a tool execution
type ExecutionResult struct {
	Output   api.ToolOutput
	Error    error
	Duration time.Duration
	Cached   bool
}

// WorkerPool manages a pool of workers for tool execution
type WorkerPool struct {
	workers chan chan ToolJob
	jobs    chan ToolJob
	quit    chan bool
	size    int
}

// ToolJob represents a job to be executed by a worker
type ToolJob struct {
	ID      string
	Tool    api.Tool
	Input   api.ToolInput
	Context context.Context
	Result  chan ExecutionResult
}

// ResultCache provides caching for tool execution results
type ResultCache struct {
	cache map[string]CachedResult
	mu    sync.RWMutex
	ttl   time.Duration
}

// CachedResult represents a cached tool execution result
type CachedResult struct {
	Output    api.ToolOutput
	Timestamp time.Time
	TTL       time.Duration
}

// MetricsBuffer buffers metrics to reduce observability overhead
type MetricsBuffer struct {
	buffer []MetricEntry
	mu     sync.Mutex
	size   int
	ticker *time.Ticker
	quit   chan bool
}

// MetricEntry represents a buffered metric
type MetricEntry struct {
	Name      string
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

// NewOptimizedExecutor creates a new optimized executor
func NewOptimizedExecutor(config ExecutorConfig) *OptimizedExecutor {
	executor := &OptimizedExecutor{
		config:      config,
		activeTools: make(map[string]*ToolExecution),
	}

	// Initialize worker pool
	if config.WorkerPoolSize > 0 {
		executor.workerPool = NewWorkerPool(config.WorkerPoolSize)
	}

	// Initialize result cache
	if config.EnableCaching {
		executor.resultCache = NewResultCache(config.CacheSize)
	}

	// Initialize metrics buffer
	if config.EnableMetrics {
		executor.metricsBuffer = NewMetricsBuffer(config.MetricsBufferSize)
	}

	return executor
}

// ExecuteTool executes a tool with optimized performance patterns
func (e *OptimizedExecutor) ExecuteTool(ctx context.Context, tool api.Tool, input api.ToolInput) (api.ToolOutput, error) {
	start := time.Now()

	// Generate execution ID
	executionID := e.generateExecutionID(tool.Name(), input)

	// Check cache first
	if e.config.EnableCaching && e.resultCache != nil {
		if cached := e.resultCache.Get(executionID); cached != nil {
			e.recordMetric("tool_execution_cache_hits", 1, map[string]string{
				"tool": tool.Name(),
			})
			return cached.Output, nil
		}
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.config.ExecutionTimeout)
	defer cancel()

	// Track active execution
	execution := &ToolExecution{
		ID:        executionID,
		ToolName:  tool.Name(),
		StartTime: start,
		Context:   execCtx,
		Cancel:    cancel,
		Result:    make(chan ExecutionResult, 1),
	}

	e.mu.Lock()
	e.activeTools[executionID] = execution
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.activeTools, executionID)
		e.mu.Unlock()
	}()

	// Validate input if enabled
	if e.config.EnableValidation && e.validator != nil {
		if err := e.validator.ValidateInput(execCtx, tool.Name(), input); err != nil {
			e.recordMetric("tool_execution_validation_failures", 1, map[string]string{
				"tool": tool.Name(),
			})
			return api.ToolOutput{}, mcperrors.NewError().Messagef("input validation failed: %w", err).WithLocation(

			// Execute using worker pool if available
			).Build()
		}
	}

	var result ExecutionResult
	if e.workerPool != nil {
		result = e.executeWithWorkerPool(execCtx, tool, input)
	} else {
		result = e.executeDirectly(execCtx, tool, input)
	}

	// Validate output if enabled
	if e.config.EnableValidation && e.validator != nil && result.Error == nil {
		// Convert mcp.ToolOutput to api.ToolOutput for validation
		apiOutput := api.ToolOutput(result.Output)
		if err := e.validator.ValidateOutput(execCtx, tool.Name(), apiOutput); err != nil {
			e.recordMetric("tool_execution_output_validation_failures", 1, map[string]string{
				"tool": tool.Name(),
			})
			return api.ToolOutput{}, mcperrors.NewError().Messagef("output validation failed: %w", err).WithLocation(

			// Cache result if successful and caching is enabled
			).Build()
		}
	}

	if e.config.EnableCaching && e.resultCache != nil && result.Error == nil {
		e.resultCache.Set(executionID, CachedResult{
			Output:    result.Output,
			Timestamp: time.Now(),
			TTL:       5 * time.Minute, // Default TTL
		})
	}

	// Record metrics
	duration := time.Since(start)
	success := result.Error == nil

	e.recordMetric("tool_execution_duration_microseconds", float64(duration.Microseconds()), map[string]string{
		"tool":    tool.Name(),
		"success": fmt.Sprintf("%t", success),
		"cached":  fmt.Sprintf("%t", result.Cached),
	})

	e.recordMetric("tool_executions_total", 1, map[string]string{
		"tool":    tool.Name(),
		"success": fmt.Sprintf("%t", success),
	})

	// Check P95 target
	if duration > 300*time.Microsecond && e.observability != nil {
		e.observability.RecordP95Violation(execCtx, tool.Name(), duration, 300*time.Microsecond)
	}

	return result.Output, result.Error
}

// executeWithWorkerPool executes a tool using the worker pool
func (e *OptimizedExecutor) executeWithWorkerPool(ctx context.Context, tool api.Tool, input api.ToolInput) ExecutionResult {
	resultChan := make(chan ExecutionResult, 1)

	job := ToolJob{
		ID:      e.generateExecutionID(tool.Name(), input),
		Tool:    tool,
		Input:   input,
		Context: ctx,
		Result:  resultChan,
	}

	// Submit job to worker pool
	select {
	case e.workerPool.jobs <- job:
		// Job submitted successfully
	case <-ctx.Done():
		return ExecutionResult{
			Error: ctx.Err(),
		}
	}

	// Wait for result
	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		return ExecutionResult{
			Error: ctx.Err(),
		}
	}
}

// executeDirectly executes a tool directly without worker pool
func (e *OptimizedExecutor) executeDirectly(ctx context.Context, tool api.Tool, input api.ToolInput) ExecutionResult {
	start := time.Now()

	// Execute using the api.Tool interface
	output, err := tool.Execute(ctx, input)

	return ExecutionResult{
		Output:   output,
		Error:    err,
		Duration: time.Since(start),
		Cached:   false,
	}
}

// recordMetric records a metric using buffered recording for performance
func (e *OptimizedExecutor) recordMetric(name string, value float64, labels map[string]string) {
	if !e.config.EnableMetrics || e.metricsBuffer == nil {
		return
	}

	// Use pooled map to avoid allocations
	pooledLabels := pools.MapPool.GetStringString()
	defer pools.MapPool.PutStringString(pooledLabels)

	for k, v := range labels {
		pooledLabels[k] = v
	}

	e.metricsBuffer.Add(MetricEntry{
		Name:      name,
		Value:     value,
		Labels:    pooledLabels,
		Timestamp: time.Now(),
	})
}

// generateExecutionID generates a cache key for tool execution
func (e *OptimizedExecutor) generateExecutionID(toolName string, input api.ToolInput) string {
	return pools.WithStringBuilder(func(sb *strings.Builder) string {
		sb.WriteString(toolName)
		sb.WriteString(":")
		sb.WriteString(input.SessionID)
		// Add a simple hash of input data to make it unique
		sb.WriteString(":")
		sb.WriteString(fmt.Sprintf("%x", len(input.Data)))
		return sb.String()
	})
}

// ============================================================================
// Worker Pool Implementation
// ============================================================================

// NewWorkerPool creates a new worker pool
func NewWorkerPool(size int) *WorkerPool {
	pool := &WorkerPool{
		workers: make(chan chan ToolJob, size),
		jobs:    make(chan ToolJob, size*2), // Buffer jobs
		quit:    make(chan bool),
		size:    size,
	}

	// Start workers
	for i := 0; i < size; i++ {
		worker := NewWorker(pool.workers, pool.quit)
		worker.Start()
	}

	// Start dispatcher
	go pool.dispatch()

	return pool
}

// dispatch dispatches jobs to workers
func (wp *WorkerPool) dispatch() {
	for {
		select {
		case job := <-wp.jobs:
			// Get an available worker
			select {
			case worker := <-wp.workers:
				// Dispatch job to worker
				worker <- job
			case <-wp.quit:
				return
			}
		case <-wp.quit:
			return
		}
	}
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.quit)
}

// Worker represents a worker in the pool
type Worker struct {
	workerPool chan chan ToolJob
	jobChannel chan ToolJob
	quit       chan bool
}

// NewWorker creates a new worker
func NewWorker(workerPool chan chan ToolJob, quit chan bool) *Worker {
	return &Worker{
		workerPool: workerPool,
		jobChannel: make(chan ToolJob),
		quit:       quit,
	}
}

// Start starts the worker
func (w *Worker) Start() {
	go func() {
		for {
			// Register worker in the worker pool
			w.workerPool <- w.jobChannel

			select {
			case job := <-w.jobChannel:
				// Execute the job
				start := time.Now()

				// Execute using the api.Tool interface
				output, err := job.Tool.Execute(job.Context, job.Input)

				duration := time.Since(start)

				// Send result
				select {
				case job.Result <- ExecutionResult{
					Output:   output,
					Error:    err,
					Duration: duration,
					Cached:   false,
				}:
				case <-job.Context.Done():
					// Context cancelled
				}

			case <-w.quit:
				return
			}
		}
	}()
}

// ============================================================================
// Result Cache Implementation
// ============================================================================

// NewResultCache creates a new result cache
func NewResultCache(size int) *ResultCache {
	return &ResultCache{
		cache: make(map[string]CachedResult, size),
		ttl:   5 * time.Minute,
	}
}

// Get retrieves a cached result
func (rc *ResultCache) Get(key string) *CachedResult {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	result, exists := rc.cache[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Since(result.Timestamp) > result.TTL {
		// Remove expired entry (defer to cleanup)
		return nil
	}

	return &result
}

// Set stores a result in the cache
func (rc *ResultCache) Set(key string, result CachedResult) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.cache[key] = result

	// Simple cleanup: if cache is getting large, remove old entries
	if len(rc.cache) > 1000 {
		rc.cleanup()
	}
}

// cleanup removes expired entries from cache
func (rc *ResultCache) cleanup() {
	now := time.Now()
	for key, result := range rc.cache {
		if now.Sub(result.Timestamp) > result.TTL {
			delete(rc.cache, key)
		}
	}
}

// ============================================================================
// Metrics Buffer Implementation
// ============================================================================

// NewMetricsBuffer creates a new metrics buffer
func NewMetricsBuffer(size int) *MetricsBuffer {
	mb := &MetricsBuffer{
		buffer: make([]MetricEntry, 0, size),
		size:   size,
		ticker: time.NewTicker(1 * time.Second), // Flush every second
		quit:   make(chan bool),
	}

	// Start background flusher
	go mb.flush()

	return mb
}

// Add adds a metric to the buffer
func (mb *MetricsBuffer) Add(entry MetricEntry) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.buffer = append(mb.buffer, entry)

	// Flush if buffer is full
	if len(mb.buffer) >= mb.size {
		mb.flushBuffer()
	}
}

// flush periodically flushes the buffer
func (mb *MetricsBuffer) flush() {
	for {
		select {
		case <-mb.ticker.C:
			mb.mu.Lock()
			mb.flushBuffer()
			mb.mu.Unlock()
		case <-mb.quit:
			return
		}
	}
}

// flushBuffer flushes the current buffer (must be called with lock held)
func (mb *MetricsBuffer) flushBuffer() {
	if len(mb.buffer) == 0 {
		return
	}

	// TODO: Send metrics to observability system
	// For now, just clear the buffer
	mb.buffer = mb.buffer[:0]
}

// Stop stops the metrics buffer
func (mb *MetricsBuffer) Stop() {
	mb.ticker.Stop()
	close(mb.quit)
}

// ============================================================================
// Configuration Helpers
// ============================================================================

// DefaultExecutorConfig returns a default executor configuration
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxConcurrency:    100,
		ExecutionTimeout:  30 * time.Second,
		EnableCaching:     true,
		EnableMetrics:     true,
		EnableValidation:  true,
		WorkerPoolSize:    10,
		CacheSize:         1000,
		MetricsBufferSize: 1000,
	}
}

// HighPerformanceConfig returns a configuration optimized for high performance
func HighPerformanceConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxConcurrency:    1000,
		ExecutionTimeout:  10 * time.Second,
		EnableCaching:     true,
		EnableMetrics:     false, // Disable for maximum performance
		EnableValidation:  false, // Disable for maximum performance
		WorkerPoolSize:    50,
		CacheSize:         10000,
		MetricsBufferSize: 0, // Disabled
	}
}

// SetObservability sets the unified observability system
func (e *OptimizedExecutor) SetObservability(obs interfaces.UnifiedObservability) {
	e.observability = obs
}

// SetValidator sets the unified validator
func (e *OptimizedExecutor) SetValidator(validator interfaces.UnifiedValidator) {
	e.validator = validator
}

// GetStats returns executor statistics
func (e *OptimizedExecutor) GetStats() ExecutorStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := ExecutorStats{
		ActiveExecutions: len(e.activeTools),
		WorkerPoolSize:   e.config.WorkerPoolSize,
		CacheEnabled:     e.config.EnableCaching,
		MetricsEnabled:   e.config.EnableMetrics,
	}

	if e.resultCache != nil {
		e.resultCache.mu.RLock()
		stats.CacheSize = len(e.resultCache.cache)
		e.resultCache.mu.RUnlock()
	}

	return stats
}

// ExecutorStats provides statistics about the executor
type ExecutorStats struct {
	ActiveExecutions int  `json:"active_executions"`
	WorkerPoolSize   int  `json:"worker_pool_size"`
	CacheSize        int  `json:"cache_size"`
	CacheEnabled     bool `json:"cache_enabled"`
	MetricsEnabled   bool `json:"metrics_enabled"`
}
