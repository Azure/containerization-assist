package common

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Lifecycle manages goroutine lifecycle with proper context handling
type Lifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	state  LifecycleState
}

// LifecycleState represents the current state of the lifecycle manager
type LifecycleState int

const (
	StateInitialized LifecycleState = iota
	StateRunning
	StateShuttingDown
	StateStopped
)

// NewLifecycle creates a new lifecycle manager
func NewLifecycle() *Lifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &Lifecycle{
		ctx:    ctx,
		cancel: cancel,
		state:  StateInitialized,
	}
}

// NewLifecycleWithContext creates a new lifecycle manager with a parent context
func NewLifecycleWithContext(parent context.Context) *Lifecycle {
	ctx, cancel := context.WithCancel(parent)
	return &Lifecycle{
		ctx:    ctx,
		cancel: cancel,
		state:  StateInitialized,
	}
}

// Context returns the lifecycle's context
func (l *Lifecycle) Context() context.Context {
	return l.ctx
}

// Go starts a new goroutine with lifecycle management
func (l *Lifecycle) Go(fn func(context.Context)) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.state != StateInitialized && l.state != StateRunning {
		return errors.Internal("lifecycle", "lifecycle is not running")
	}

	l.state = StateRunning
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()
	return nil
}

// GoWithRecover starts a new goroutine with panic recovery
func (l *Lifecycle) GoWithRecover(fn func(context.Context), onPanic func(interface{})) error {
	return l.Go(func(ctx context.Context) {
		defer func() {
			if r := recover(); r != nil && onPanic != nil {
				onPanic(r)
			}
		}()
		fn(ctx)
	})
}

// Shutdown gracefully shuts down all managed goroutines
func (l *Lifecycle) Shutdown(timeout time.Duration) error {
	l.mu.Lock()
	if l.state == StateShuttingDown || l.state == StateStopped {
		l.mu.Unlock()
		return errors.Internal("lifecycle", "lifecycle already shutting down or stopped")
	}
	l.state = StateShuttingDown
	l.mu.Unlock()

	// Cancel context to signal shutdown
	l.cancel()

	// Wait for all goroutines to complete with timeout
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		l.mu.Lock()
		l.state = StateStopped
		l.mu.Unlock()
		return nil
	case <-time.After(timeout):
		l.mu.Lock()
		l.state = StateStopped
		l.mu.Unlock()
		return errors.Internal("lifecycle", "shutdown timeout exceeded")
	}
}

// IsRunning returns true if the lifecycle is still running
func (l *Lifecycle) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state == StateInitialized || l.state == StateRunning
}

// WorkerPool provides bounded concurrency with lifecycle management
type WorkerPool struct {
	workers   int
	taskQueue chan func()
	lifecycle *Lifecycle
	mu        sync.Mutex
	closed    bool
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}

	wp := &WorkerPool{
		workers:   workers,
		taskQueue: make(chan func(), workers*2), // Buffered channel
		lifecycle: NewLifecycle(),
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wp.lifecycle.Go(wp.worker)
	}

	return wp
}

// NewWorkerPoolWithContext creates a new worker pool with a parent context
func NewWorkerPoolWithContext(parent context.Context, workers int) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}

	wp := &WorkerPool{
		workers:   workers,
		taskQueue: make(chan func(), workers*2),
		lifecycle: NewLifecycleWithContext(parent),
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wp.lifecycle.Go(wp.worker)
	}

	return wp
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(task func()) error {
	wp.mu.Lock()
	if wp.closed {
		wp.mu.Unlock()
		return errors.Internal("lifecycle", "worker pool is closed")
	}
	wp.mu.Unlock()

	select {
	case wp.taskQueue <- task:
		return nil
	case <-wp.lifecycle.ctx.Done():
		return errors.Internal("lifecycle", "worker pool shut down")
	}
}

// SubmitWithTimeout submits a task with a timeout
func (wp *WorkerPool) SubmitWithTimeout(task func(), timeout time.Duration) error {
	wp.mu.Lock()
	if wp.closed {
		wp.mu.Unlock()
		return errors.Internal("lifecycle", "worker pool is closed")
	}
	wp.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case wp.taskQueue <- task:
		return nil
	case <-timer.C:
		return errors.Internal("lifecycle", "submission timeout")
	case <-wp.lifecycle.ctx.Done():
		return errors.Internal("lifecycle", "worker pool shut down")
	}
}

func (wp *WorkerPool) worker(ctx context.Context) {
	for {
		select {
		case task := <-wp.taskQueue:
			if task != nil {
				task()
			}
		case <-ctx.Done():
			return
		}
	}
}

// Shutdown gracefully shuts down the worker pool
func (wp *WorkerPool) Shutdown(timeout time.Duration) error {
	wp.mu.Lock()
	if wp.closed {
		wp.mu.Unlock()
		return errors.Internal("lifecycle", "worker pool already closed")
	}
	wp.closed = true
	close(wp.taskQueue)
	wp.mu.Unlock()

	return wp.lifecycle.Shutdown(timeout)
}

// Size returns the number of workers in the pool
func (wp *WorkerPool) Size() int {
	return wp.workers
}

// QueueSize returns the current number of tasks in the queue
func (wp *WorkerPool) QueueSize() int {
	return len(wp.taskQueue)
}

// BackgroundWorker interface for managed background workers
type BackgroundWorker interface {
	Start(ctx context.Context) error
	Stop() error
	Health() WorkerHealth
}

// WorkerHealth represents the health status of a worker
type WorkerHealth struct {
	Status    string
	LastCheck time.Time
	Error     error
}

// WorkerManager manages multiple background workers
type WorkerManager struct {
	workers   map[string]BackgroundWorker
	lifecycle *Lifecycle
	mu        sync.RWMutex
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager() *WorkerManager {
	return &WorkerManager{
		workers:   make(map[string]BackgroundWorker),
		lifecycle: NewLifecycle(),
	}
}

// RegisterWorker registers a new background worker
func (m *WorkerManager) RegisterWorker(name string, worker BackgroundWorker) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.workers[name]; exists {
		return errors.Internal("lifecycle", "worker already registered: "+name)
	}

	m.workers[name] = worker
	return nil
}

// StartAll starts all registered workers
func (m *WorkerManager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, worker := range m.workers {
		// Start each worker in its own goroutine
		workerInstance := worker
		if err := m.lifecycle.Go(func(ctx context.Context) {
			if err := workerInstance.Start(ctx); err != nil {
				// Log error but don't fail other workers
				// In real implementation, use proper logging
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all workers gracefully
func (m *WorkerManager) StopAll(timeout time.Duration) error {
	m.mu.RLock()
	workers := make([]BackgroundWorker, 0, len(m.workers))
	for _, worker := range m.workers {
		workers = append(workers, worker)
	}
	m.mu.RUnlock()

	// Stop all workers
	var wg sync.WaitGroup
	errChan := make(chan error, len(workers))

	for _, worker := range workers {
		wg.Add(1)
		go func(w BackgroundWorker) {
			defer wg.Done()
			if err := w.Stop(); err != nil {
				errChan <- err
			}
		}(worker)
	}

	// Wait for all workers to stop
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers stopped successfully
	case <-time.After(timeout):
		return errors.Internal("lifecycle", "worker shutdown timeout")
	}

	// Shutdown lifecycle manager
	return m.lifecycle.Shutdown(timeout)
}

// HealthCheck returns health status for all workers
func (m *WorkerManager) HealthCheck() map[string]WorkerHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := make(map[string]WorkerHealth)
	for name, worker := range m.workers {
		health[name] = worker.Health()
	}
	return health
}

// GetWorker returns a specific worker by name
func (m *WorkerManager) GetWorker(name string) (BackgroundWorker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	worker, exists := m.workers[name]
	return worker, exists
}

// FailureAnalyzer provides failure analysis capabilities for operations
type FailureAnalyzer struct {
	maxRetries    int
	retryDelay    time.Duration
	failureTypes  map[string]bool
	analysisDepth int
}

// NewFailureAnalyzer creates a new failure analyzer
func NewFailureAnalyzer() *FailureAnalyzer {
	return &FailureAnalyzer{
		maxRetries:    3,
		retryDelay:    time.Second,
		failureTypes:  make(map[string]bool),
		analysisDepth: 5,
	}
}

// AnalyzeFailure analyzes a failure and determines if it's retryable
func (fa *FailureAnalyzer) AnalyzeFailure(err error) (bool, string) {
	if err == nil {
		return false, ""
	}

	errorMsg := err.Error()

	// Check for known retryable patterns
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"temporary failure",
		"resource temporarily unavailable",
		"network error",
		"service unavailable",
	}

	for _, pattern := range retryablePatterns {
		if containsPattern(errorMsg, pattern) {
			return true, fmt.Sprintf("retryable error detected: %s", pattern)
		}
	}

	// Non-retryable patterns
	nonRetryablePatterns := []string{
		"permission denied",
		"authentication failed",
		"invalid credentials",
		"not found",
		"bad request",
		"forbidden",
	}

	for _, pattern := range nonRetryablePatterns {
		if containsPattern(errorMsg, pattern) {
			return false, fmt.Sprintf("non-retryable error detected: %s", pattern)
		}
	}

	// Default to non-retryable for unknown errors
	return false, "unknown error type"
}

// ShouldRetry determines if an operation should be retried based on attempt count
func (fa *FailureAnalyzer) ShouldRetry(err error, attemptCount int) bool {
	if attemptCount >= fa.maxRetries {
		return false
	}

	retryable, _ := fa.AnalyzeFailure(err)
	return retryable
}

// GetRetryDelay returns the delay before the next retry attempt
func (fa *FailureAnalyzer) GetRetryDelay(attemptCount int) time.Duration {
	// Exponential backoff
	delay := fa.retryDelay * time.Duration(1<<uint(attemptCount))
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func containsPattern(text, pattern string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}

// PathUtils provides path validation utilities
type PathUtils struct{}

// NewPathUtils creates a new PathUtils instance
func NewPathUtils() *PathUtils {
	return &PathUtils{}
}

// ValidateLocalPath validates a local path
func (p *PathUtils) ValidateLocalPath(path string) error {
	if path == "" {
		return errors.NewError().Messagef("path cannot be empty").WithLocation().Build()
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return errors.NewError().Messagef("path does not exist: %s", path).WithLocation().Build()
		}
		return errors.NewError().Messagef("cannot access path: %s", path).WithLocation().Build()
	}

	info, err := os.Stat(path)
	if err != nil {
		return errors.NewError().Messagef("cannot stat path: %s", path).WithLocation().Build()
	}

	if !info.IsDir() {
		return errors.NewError().Messagef("path is not a directory: %s", path).WithLocation().Build()
	}

	return nil
}
