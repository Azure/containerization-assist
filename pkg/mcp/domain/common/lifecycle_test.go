package common

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifecycle_NoGoroutineLeaks(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	lifecycle := NewLifecycle()

	var counter int64
	for i := 0; i < 10; i++ {
		err := lifecycle.Go(func(ctx context.Context) {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					atomic.AddInt64(&counter, 1)
				}
			}
		})
		if err != nil {
			t.Fatalf("Failed to start goroutine: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(&counter) == 0 {
		t.Error("Goroutines don't appear to be running")
	}

	err := lifecycle.Shutdown(5 * time.Second)
	if err != nil {
		t.Fatalf("Failed to shutdown lifecycle: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines {
		t.Errorf("Goroutine leak detected: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}

func TestLifecycle_ShutdownTimeout(t *testing.T) {
	lifecycle := NewLifecycle()

	err := lifecycle.Go(func(ctx context.Context) {
		time.Sleep(10 * time.Second)
	})
	if err != nil {
		t.Fatalf("Failed to start goroutine: %v", err)
	}

	start := time.Now()
	err = lifecycle.Shutdown(100 * time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected shutdown to timeout")
	}

	if duration < 100*time.Millisecond || duration > 200*time.Millisecond {
		t.Errorf("Shutdown timeout took %v, expected ~100ms", duration)
	}
}

func TestLifecycle_ContextCancellation(t *testing.T) {
	lifecycle := NewLifecycle()

	var cancelled bool
	err := lifecycle.Go(func(ctx context.Context) {
		<-ctx.Done()
		cancelled = true
	})
	if err != nil {
		t.Fatalf("Failed to start goroutine: %v", err)
	}

	err = lifecycle.Shutdown(1 * time.Second)
	if err != nil {
		t.Fatalf("Failed to shutdown: %v", err)
	}

	if !cancelled {
		t.Error("Context was not cancelled")
	}
}

func TestLifecycle_PanicRecovery(t *testing.T) {
	lifecycle := NewLifecycle()

	var panicCaught atomic.Value
	err := lifecycle.GoWithRecover(
		func(ctx context.Context) {
			panic("test panic")
		},
		func(r interface{}) {
			panicCaught.Store(r)
		},
	)
	if err != nil {
		t.Fatalf("Failed to start goroutine: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	caught := panicCaught.Load()
	if caught != "test panic" {
		t.Errorf("Expected panic to be caught, got: %v", caught)
	}

	err = lifecycle.Shutdown(1 * time.Second)
	if err != nil {
		t.Fatalf("Failed to shutdown after panic: %v", err)
	}
}

func TestWorkerpool_NoGoroutineLeaks(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	// Create worker pool
	pool := NewWorkerPool(5)

	// Submit tasks
	var counter int64
	for i := 0; i < 100; i++ {
		err := pool.Submit(func() {
			atomic.AddInt64(&counter, 1)
			time.Sleep(1 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}

	// Wait for tasks to complete
	time.Sleep(200 * time.Millisecond)

	// Verify tasks were executed
	if atomic.LoadInt64(&counter) != 100 {
		t.Errorf("Expected 100 tasks to complete, got %d", atomic.LoadInt64(&counter))
	}

	// Shutdown pool
	err := pool.Shutdown(5 * time.Second)
	if err != nil {
		t.Fatalf("Failed to shutdown pool: %v", err)
	}

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines {
		t.Errorf("Goroutine leak detected: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}

func TestWorkerPool_SubmissionAfterShutdown(t *testing.T) {
	pool := NewWorkerPool(2)

	// Shutdown pool
	err := pool.Shutdown(1 * time.Second)
	if err != nil {
		t.Fatalf("Failed to shutdown pool: %v", err)
	}

	// Try to submit task after shutdown
	err = pool.Submit(func() {
		// This should not execute
	})

	if err == nil {
		t.Error("Expected error when submitting task to closed pool")
	}
}

func TestWorkerPool_SubmitWithTimeout(t *testing.T) {
	// Create small pool to test backpressure
	pool := NewWorkerPool(1)

	// Fill the queue completely by submitting more tasks than the buffer size
	queueSize := pool.workers * 2 // This is the queue buffer size
	for i := 0; i < queueSize; i++ {
		err := pool.Submit(func() {
			time.Sleep(200 * time.Millisecond) // Longer sleep to ensure queue stays full
		})
		if err != nil {
			t.Fatalf("Failed to submit task %d: %v", i, err)
		}
	}

	// Add one more task to fill the worker
	err := pool.Submit(func() {
		time.Sleep(200 * time.Millisecond)
	})
	if err != nil {
		t.Fatalf("Failed to submit worker task: %v", err)
	}

	// Now the queue should be full, this should timeout
	start := time.Now()
	err = pool.SubmitWithTimeout(func() {}, 100*time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if duration < 80*time.Millisecond {
		t.Errorf("Timeout happened too quickly: %v (expected ~100ms)", duration)
	}

	// Cleanup
	pool.Shutdown(5 * time.Second)
}

func TestWorkerManager_WorkerRegistration(t *testing.T) {
	manager := NewWorkerManager()

	// Create mock worker
	worker := &mockWorker{name: "test-worker"}

	// Register worker
	err := manager.RegisterWorker("test-worker", worker)
	if err != nil {
		t.Fatalf("Failed to register worker: %v", err)
	}

	// Try to register same worker again
	err = manager.RegisterWorker("test-worker", worker)
	if err == nil {
		t.Error("Expected error when registering duplicate worker")
	}

	// Get worker
	retrievedWorker, exists := manager.GetWorker("test-worker")
	if !exists {
		t.Error("Worker not found after registration")
	}

	if retrievedWorker != worker {
		t.Error("Retrieved worker is not the same as registered worker")
	}
}

func TestWorkerManager_HealthChecks(t *testing.T) {
	manager := NewWorkerManager()

	// Create mock worker
	worker := &mockWorker{
		name:   "test-worker",
		health: WorkerHealth{Status: "healthy"},
	}

	// Register and start
	err := manager.RegisterWorker("test-worker", worker)
	if err != nil {
		t.Fatalf("Failed to register worker: %v", err)
	}

	err = manager.StartAll()
	if err != nil {
		t.Fatalf("Failed to start workers: %v", err)
	}

	// Wait for health check
	time.Sleep(100 * time.Millisecond)

	// Check health
	health := manager.HealthCheck()
	workerHealth, exists := health["test-worker"]
	if !exists {
		t.Error("Worker health not found")
	}

	if workerHealth.Status != "healthy" {
		t.Errorf("Expected healthy status, got: %s", workerHealth.Status)
	}

	// Cleanup
	manager.StopAll(2 * time.Second)
}

// Mock worker for testing
type mockWorker struct {
	name    string
	health  WorkerHealth
	started bool
	stopped bool
}

func (w *mockWorker) Start(ctx context.Context) error {
	w.started = true
	<-ctx.Done()
	return nil
}

func (w *mockWorker) Stop() error {
	w.stopped = true
	return nil
}

func (w *mockWorker) Health() WorkerHealth {
	return w.health
}

func BenchmarkLifecycle_GoroutineCreation(b *testing.B) {
	lifecycle := NewLifecycle()
	defer lifecycle.Shutdown(5 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lifecycle.Go(func(ctx context.Context) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Millisecond):
				return
			}
		})
	}
}

func BenchmarkWorkerPool_TaskSubmission(b *testing.B) {
	pool := NewWorkerPool(10)
	defer pool.Shutdown(5 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(func() {
			// Minimal work
		})
	}
}

// Memory leak detection test
func TestMemoryLeakDetection(t *testing.T) {
	var m1, m2 runtime.MemStats

	// Force garbage collection and get baseline
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Create and destroy many lifecycles
	for i := 0; i < 100; i++ {
		lifecycle := NewLifecycle()

		for j := 0; j < 10; j++ {
			lifecycle.Go(func(ctx context.Context) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Millisecond):
					return
				}
			})
		}

		// Shutdown
		lifecycle.Shutdown(1 * time.Second)
	}

	// Force garbage collection and measure
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Check for significant memory increase
	growth := float64(m2.Alloc-m1.Alloc) / float64(m1.Alloc)
	if growth > 0.5 {
		t.Errorf("Potential memory leak detected: memory grew by %.2f%% (from %d to %d bytes)",
			growth*100, m1.Alloc, m2.Alloc)
	}
}
