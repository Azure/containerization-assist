package session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConcurrentAdapter(t *testing.T) (*ConcurrentBoltAdapter, func()) {
	tmpDir, err := os.MkdirTemp("", "concurrent_adapter_test")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	adapter, err := NewConcurrentBoltAdapter(dbPath, logger, 24*time.Hour, 100)
	require.NoError(t, err)

	cleanup := func() {
		_ = adapter.Stop(context.Background())
		_ = os.RemoveAll(tmpDir)
	}

	return adapter, cleanup
}

func TestConcurrentBoltAdapter_UpdateWorkflowState(t *testing.T) {
	adapter, cleanup := createTestConcurrentAdapter(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "workflow-test-1"

	// Create initial session
	_, err := adapter.GetOrCreate(ctx, sessionID)
	require.NoError(t, err)

	// Test concurrent workflow state updates
	var wg sync.WaitGroup
	updateCount := 100
	goroutines := 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < updateCount/goroutines; j++ {
				err := adapter.UpdateWorkflowState(ctx, sessionID, func(metadata map[string]interface{}) error {
					var counter int
					if metadata["counter"] != nil {
						// Handle JSON float64 conversion
						if val, ok := metadata["counter"].(float64); ok {
							counter = int(val)
						} else if val, ok := metadata["counter"].(int); ok {
							counter = val
						}
					}
					metadata["counter"] = counter + 1

					// Also track which workers touched the state
					if metadata["workers"] == nil {
						metadata["workers"] = make(map[string]int)
					}
					// Handle map[string]interface{} from JSON
					var workers map[string]int
					if w, ok := metadata["workers"].(map[string]interface{}); ok {
						workers = make(map[string]int)
						for k, v := range w {
							if val, ok := v.(float64); ok {
								workers[k] = int(val)
							} else if val, ok := v.(int); ok {
								workers[k] = val
							}
						}
					} else if w, ok := metadata["workers"].(map[string]int); ok {
						workers = w
					} else {
						workers = make(map[string]int)
					}

					key := fmt.Sprintf("worker-%d", workerID)
					workers[key] = workers[key] + 1
					metadata["workers"] = workers

					return nil
				})
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	finalMetadata, err := adapter.GetWorkflowState(ctx, sessionID)
	require.NoError(t, err)

	// Handle JSON float64 conversion for final counter
	var finalCounter int
	if val, ok := finalMetadata["counter"].(float64); ok {
		finalCounter = int(val)
	} else if val, ok := finalMetadata["counter"].(int); ok {
		finalCounter = val
	}
	assert.Equal(t, updateCount, finalCounter)

	// Verify all workers participated
	var workerCount int
	if w, ok := finalMetadata["workers"].(map[string]interface{}); ok {
		workerCount = len(w)
	}
	assert.Equal(t, goroutines, workerCount)
}

func TestConcurrentBoltAdapter_AcquireWorkflowLock(t *testing.T) {
	adapter, cleanup := createTestConcurrentAdapter(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "lock-test-1"

	// Create session
	_, err := adapter.GetOrCreate(ctx, sessionID)
	require.NoError(t, err)

	// Test exclusive locking
	results := make(chan string, 3)

	// Worker 1: Acquires lock and holds it
	go func() {
		unlock := adapter.AcquireWorkflowLock(sessionID)
		results <- "worker1-locked"
		time.Sleep(100 * time.Millisecond)
		unlock()
		results <- "worker1-unlocked"
	}()

	// Worker 2: Tries to acquire lock (should wait)
	go func() {
		time.Sleep(10 * time.Millisecond) // Ensure worker 1 gets lock first
		unlock := adapter.AcquireWorkflowLock(sessionID)
		results <- "worker2-locked"
		unlock()
	}()

	// Verify order of operations
	assert.Equal(t, "worker1-locked", <-results)
	assert.Equal(t, "worker1-unlocked", <-results)
	assert.Equal(t, "worker2-locked", <-results)
}

func TestConcurrentBoltAdapter_UpdateWithVersion(t *testing.T) {
	adapter, cleanup := createTestConcurrentAdapter(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "version-test-1"

	// Create initial session
	state, err := adapter.GetOrCreate(ctx, sessionID)
	require.NoError(t, err)

	originalVersion := state.UpdatedAt

	// Successful update with correct version
	err = adapter.UpdateWithVersion(ctx, sessionID, originalVersion, func(s *SessionState) error {
		if s.Metadata == nil {
			s.Metadata = make(map[string]interface{})
		}
		s.Metadata["value"] = "updated"
		return nil
	})
	assert.NoError(t, err)

	// Failed update with old version
	err = adapter.UpdateWithVersion(ctx, sessionID, originalVersion, func(s *SessionState) error {
		s.Metadata["value"] = "should-fail"
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "was modified by another process")

	// Verify value is from successful update
	finalState, err := adapter.Get(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, "updated", finalState.Metadata["value"])
}

func TestConcurrentBoltAdapter_BatchUpdateWorkflowStates(t *testing.T) {
	adapter, cleanup := createTestConcurrentAdapter(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions
	sessionIDs := []string{"batch-1", "batch-2", "batch-3"}
	for _, id := range sessionIDs {
		_, err := adapter.GetOrCreate(ctx, id)
		require.NoError(t, err)
	}

	// Prepare batch updates
	updates := make(map[string]func(map[string]interface{}) error)
	for _, id := range sessionIDs {
		sessionID := id // Capture for closure
		updates[sessionID] = func(metadata map[string]interface{}) error {
			metadata["batch_value"] = 100
			metadata["batch_id"] = sessionID
			metadata["updated_at"] = time.Now().Format(time.RFC3339)
			return nil
		}
	}

	// Execute batch update
	err := adapter.BatchUpdateWorkflowStates(ctx, updates)
	require.NoError(t, err)

	// Verify all sessions were updated
	for _, id := range sessionIDs {
		metadata, err := adapter.GetWorkflowState(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, float64(100), metadata["batch_value"])
		assert.Equal(t, id, metadata["batch_id"])
		assert.NotNil(t, metadata["updated_at"])
	}
}

func TestConcurrentBoltAdapter_CleanupLocks(t *testing.T) {
	adapter, cleanup := createTestConcurrentAdapter(t)
	defer cleanup()

	ctx := context.Background()

	// Create active and expired sessions
	activeID := "active-session"
	expiredID := "expired-session"

	// Create active session
	_, err := adapter.GetOrCreate(ctx, activeID)
	require.NoError(t, err)

	// Create expired session
	_, err = adapter.GetOrCreate(ctx, expiredID)
	require.NoError(t, err)

	// Manually expire the session
	err = adapter.Update(ctx, expiredID, func(s *SessionState) error {
		s.ExpiresAt = time.Now().Add(-1 * time.Hour) // Expired 1 hour ago
		return nil
	})
	require.NoError(t, err)

	// Acquire locks for both sessions and release them before cleanup
	unlock1 := adapter.AcquireWorkflowLock(activeID)
	unlock2 := adapter.AcquireWorkflowLock(expiredID)
	unlock1()
	unlock2()

	// Run cleanup
	adapter.CleanupLocks(ctx)

	// Verify locks state (this is implementation-specific testing)
	// The expired session lock should be removed from the map
	// We can't directly test this without exposing internals,
	// but we can verify that operations still work

	// Should be able to update active session
	err = adapter.UpdateWorkflowState(ctx, activeID, func(metadata map[string]interface{}) error {
		metadata["test"] = "active"
		return nil
	})
	assert.NoError(t, err)
}

func TestConcurrentBoltAdapter_WorkflowStateIsolation(t *testing.T) {
	adapter, cleanup := createTestConcurrentAdapter(t)
	defer cleanup()

	ctx := context.Background()

	// Test that GetWorkflowState returns a copy, not a reference
	sessionID := "isolation-test"
	_, err := adapter.GetOrCreate(ctx, sessionID)
	require.NoError(t, err)

	// Set initial state
	err = adapter.UpdateWorkflowState(ctx, sessionID, func(metadata map[string]interface{}) error {
		metadata["value"] = "original"
		metadata["nested"] = map[string]interface{}{
			"key": "value",
		}
		return nil
	})
	require.NoError(t, err)

	// Get state and modify the returned copy
	metadata1, err := adapter.GetWorkflowState(ctx, sessionID)
	require.NoError(t, err)
	metadata1["value"] = "modified"
	metadata1["nested"].(map[string]interface{})["key"] = "modified"

	// Get state again - should still have original values
	metadata2, err := adapter.GetWorkflowState(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, "original", metadata2["value"])
	assert.Equal(t, "value", metadata2["nested"].(map[string]interface{})["key"])
}

func TestConcurrentBoltAdapter_HighContention(t *testing.T) {
	adapter, cleanup := createTestConcurrentAdapter(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "high-contention"

	_, err := adapter.GetOrCreate(ctx, sessionID)
	require.NoError(t, err)

	// Initialize workflow state
	err = adapter.UpdateWorkflowState(ctx, sessionID, func(metadata map[string]interface{}) error {
		metadata["steps_completed"] = []string{}
		metadata["total_operations"] = 0
		return nil
	})
	require.NoError(t, err)

	// Simulate high contention with many workers
	var wg sync.WaitGroup
	workers := 50
	operationsPerWorker := 20

	start := time.Now()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operationsPerWorker; j++ {
				// Mix of operations
				if j%3 == 0 {
					// Read operation
					_, err := adapter.GetWorkflowState(ctx, sessionID)
					assert.NoError(t, err)
				} else {
					// Write operation
					err := adapter.UpdateWorkflowState(ctx, sessionID, func(metadata map[string]interface{}) error {
						// Handle JSON float64 conversion
						var ops int
						if val, ok := metadata["total_operations"].(float64); ok {
							ops = int(val)
						} else if val, ok := metadata["total_operations"].(int); ok {
							ops = val
						}
						metadata["total_operations"] = ops + 1

						// Handle JSON array conversion
						var steps []string
						if stepsInterface, ok := metadata["steps_completed"].([]interface{}); ok {
							for _, s := range stepsInterface {
								if str, ok := s.(string); ok {
									steps = append(steps, str)
								}
							}
						} else if stepsStr, ok := metadata["steps_completed"].([]string); ok {
							steps = stepsStr
						}
						steps = append(steps, fmt.Sprintf("worker-%d-op-%d", workerID, j))
						metadata["steps_completed"] = steps

						return nil
					})
					assert.NoError(t, err)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Verify correctness
	finalMetadata, err := adapter.GetWorkflowState(ctx, sessionID)
	require.NoError(t, err)

	// Count write operations (2 out of 3 operations are writes)
	expectedWrites := 0
	for i := 0; i < workers; i++ {
		for j := 0; j < operationsPerWorker; j++ {
			if j%3 != 0 {
				expectedWrites++
			}
		}
	}

	// Handle JSON type conversions
	var totalOps int
	if val, ok := finalMetadata["total_operations"].(float64); ok {
		totalOps = int(val)
	} else if val, ok := finalMetadata["total_operations"].(int); ok {
		totalOps = val
	}
	assert.Equal(t, expectedWrites, totalOps)

	var steps []string
	if stepsInterface, ok := finalMetadata["steps_completed"].([]interface{}); ok {
		for _, s := range stepsInterface {
			if str, ok := s.(string); ok {
				steps = append(steps, str)
			}
		}
	}
	assert.Equal(t, expectedWrites, len(steps))

	// Performance check - should complete in reasonable time
	assert.Less(t, duration, 10*time.Second, "High contention test took too long")

	t.Logf("High contention test completed in %v for %d workers with %d operations each",
		duration, workers, operationsPerWorker)
}
