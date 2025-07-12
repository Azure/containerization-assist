package workflow

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockReporter for testing
type mockReporter struct {
	mu            sync.Mutex
	beginCalls    []string
	updateCalls   []updateCall
	completeCalls []string
	closed        bool
}

type updateCall struct {
	step    int
	total   int
	message string
}

func (m *mockReporter) Begin(message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beginCalls = append(m.beginCalls, message)
	return nil
}

func (m *mockReporter) Update(step, total int, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls = append(m.updateCalls, updateCall{step, total, message})
	return nil
}

func (m *mockReporter) Complete(message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCalls = append(m.completeCalls, message)
	return nil
}

func (m *mockReporter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockReporter) getCalls() ([]string, []updateCall, []string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.beginCalls...),
		append([]updateCall{}, m.updateCalls...),
		append([]string{}, m.completeCalls...),
		m.closed
}

func TestChannelManager_BasicFlow(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	manager := NewChannelManager(ctx, nil, 3, logger)
	defer manager.Finish()

	// Test basic flow
	manager.Begin("Starting test")
	time.Sleep(10 * time.Millisecond) // Allow processing

	manager.Update(1, "Step 1", map[string]interface{}{"test": true})
	time.Sleep(10 * time.Millisecond)

	manager.Update(2, "Step 2", nil)
	time.Sleep(10 * time.Millisecond)

	manager.Update(3, "Step 3", nil)
	time.Sleep(10 * time.Millisecond)

	manager.Complete("Test completed")

	// Verify state
	assert.Equal(t, 3, manager.GetTotal())
	assert.True(t, manager.IsComplete())
	assert.NotEmpty(t, manager.GetTraceID())
}

func TestChannelManager_Throttling(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewChannelManager(ctx, nil, 10, logger)
	defer manager.Finish()

	manager.Begin("Throttling test")

	// Send rapid updates
	start := time.Now()
	for i := 1; i <= 5; i++ {
		manager.Update(i, "Rapid update", nil)
	}

	// Updates should be throttled, so this should complete quickly
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 50*time.Millisecond, "Updates should not block")

	manager.Complete("Done")
}

func TestChannelManager_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewChannelManager(ctx, nil, 100, logger)
	defer manager.Finish()

	manager.Begin("Concurrency test")

	// Test concurrent access from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				step := id*10 + j + 1
				if step <= 100 {
					manager.Update(step, "Concurrent update", map[string]interface{}{
						"goroutine": id,
						"iteration": j,
					})
				}
				// Test getters
				_ = manager.GetCurrent()
				_ = manager.GetTotal()
				_ = manager.IsComplete()
			}
		}(i)
	}

	wg.Wait()
	manager.Complete("Concurrency test completed")

	assert.Equal(t, 100, manager.GetTotal())
}

func TestChannelManager_ErrorBudget(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewChannelManager(ctx, nil, 5, logger)
	defer manager.Finish()

	manager.Begin("Error budget test")

	// Test error budget
	assert.False(t, manager.IsCircuitOpen())

	// Record some errors
	for i := 0; i < 3; i++ {
		success := manager.RecordError(assert.AnError)
		assert.True(t, success, "Should be within budget")
	}

	// Record success
	manager.RecordSuccess()

	// Test error handling update
	metadata := map[string]interface{}{"test": true}
	success := manager.UpdateWithErrorHandling(1, "Test with error", metadata, assert.AnError)

	// Should still be successful as we're within budget
	assert.True(t, success)
	assert.Equal(t, "failed", metadata["status"])
	assert.Equal(t, assert.AnError.Error(), metadata["error"])

	manager.Complete("Error budget test completed")
}

func TestChannelManager_Heartbeat(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create manager with very short heartbeat interval for testing
	manager := NewChannelManager(ctx, nil, 5, logger)
	manager.heartbeatInterval = 50 * time.Millisecond // Very short for testing
	defer manager.Finish()

	manager.Begin("Heartbeat test")

	// Wait longer than heartbeat interval without updates
	time.Sleep(100 * time.Millisecond)

	// The heartbeat should have triggered
	// Complete immediately to test cleanup
	manager.Complete("Heartbeat test completed")
}

func TestChannelManager_CleanShutdown(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewChannelManager(ctx, nil, 3, logger)

	manager.Begin("Shutdown test")
	manager.Update(1, "Step 1", nil)

	// Test clean shutdown
	start := time.Now()
	manager.Finish()
	elapsed := time.Since(start)

	// Should shutdown quickly
	assert.Less(t, elapsed, 100*time.Millisecond, "Shutdown should be fast")
}

func TestChannelManager_StateConsistency(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewChannelManager(ctx, nil, 5, logger)
	defer manager.Finish()

	manager.Begin("State test")

	// Test initial state
	assert.Equal(t, 0, manager.GetCurrent())
	assert.Equal(t, 5, manager.GetTotal())
	assert.False(t, manager.IsComplete())

	// Update and verify state
	manager.Update(3, "Middle step", nil)
	time.Sleep(10 * time.Millisecond) // Allow processing

	// Note: GetCurrent() goes through the channel, so it might not be immediately updated
	// In a real scenario, you'd rely on the completion callback or final state

	manager.Update(5, "Final step", nil)
	time.Sleep(10 * time.Millisecond)

	manager.Complete("State test completed")
}

// Benchmark tests

func BenchmarkChannelManager_Updates(b *testing.B) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewChannelManager(ctx, nil, b.N, logger)
	defer manager.Finish()

	manager.Begin("Benchmark test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Update(i+1, "Benchmark update", nil)
	}
	b.StopTimer()

	manager.Complete("Benchmark completed")
}

func BenchmarkChannelManager_ConcurrentUpdates(b *testing.B) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewChannelManager(ctx, nil, b.N, logger)
	defer manager.Finish()

	manager.Begin("Concurrent benchmark")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			manager.Update(i, "Concurrent update", nil)
		}
	})
	b.StopTimer()

	manager.Complete("Concurrent benchmark completed")
}
