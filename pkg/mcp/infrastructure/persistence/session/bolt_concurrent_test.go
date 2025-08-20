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

	"github.com/Azure/containerization-assist/pkg/mcp/domain/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConcurrentStore(t *testing.T) (*ConcurrentBoltStore, func()) {
	tmpDir, err := os.MkdirTemp("", "bolt_concurrent_test")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	baseStore, err := NewBoltStore(dbPath, logger)
	require.NoError(t, err)

	store := NewConcurrentBoltStore(baseStore)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestConcurrentBoltStore_UpdateAtomic(t *testing.T) {
	store, cleanup := createTestConcurrentStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "test-session-1"

	// Create initial session
	sess := session.Session{
		ID:        sessionID,
		UserID:    "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    session.StatusActive,
		Labels:    make(map[string]string),
		Metadata:  map[string]interface{}{"counter": 0},
	}
	err := store.Create(ctx, sess)
	require.NoError(t, err)

	// Test concurrent atomic updates
	var wg sync.WaitGroup
	updateCount := 100
	goroutines := 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < updateCount/goroutines; j++ {
				err := store.UpdateAtomic(ctx, sessionID, func(s *session.Session) error {
					// JSON unmarshaling converts numbers to float64
					var counter int
					if val, ok := s.Metadata["counter"].(float64); ok {
						counter = int(val)
					} else if val, ok := s.Metadata["counter"].(int); ok {
						counter = val
					}
					s.Metadata["counter"] = counter + 1
					return nil
				})
				assert.NoError(t, err)
			}
		}()
	}

	wg.Wait()

	// Verify final count
	finalSess, err := store.Get(ctx, sessionID)
	require.NoError(t, err)
	// JSON unmarshaling converts numbers to float64
	finalCount := int(finalSess.Metadata["counter"].(float64))
	assert.Equal(t, updateCount, finalCount)
}

func TestConcurrentBoltStore_CompareAndSwap(t *testing.T) {
	store, cleanup := createTestConcurrentStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "test-session-cas"

	// Create initial session
	sess := session.Session{
		ID:        sessionID,
		UserID:    "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    session.StatusActive,
		Labels:    make(map[string]string),
		Metadata:  map[string]interface{}{"value": "initial"},
	}
	err := store.Create(ctx, sess)
	require.NoError(t, err)

	// Get current version
	currentSess, err := store.Get(ctx, sessionID)
	require.NoError(t, err)
	version := currentSess.UpdatedAt.UnixNano()

	// Successful CAS with correct version
	err = store.CompareAndSwap(ctx, sessionID, version, func(s *session.Session) error {
		s.Metadata["value"] = "updated"
		return nil
	})
	assert.NoError(t, err)

	// Failed CAS with old version (should fail)
	err = store.CompareAndSwap(ctx, sessionID, version, func(s *session.Session) error {
		s.Metadata["value"] = "should-not-update"
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "was modified by another process")

	// Verify value is from successful update
	finalSess, err := store.Get(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, "updated", finalSess.Metadata["value"])
}

func TestConcurrentBoltStore_GetWithLock(t *testing.T) {
	store, cleanup := createTestConcurrentStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "test-session-lock"

	// Create initial session
	sess := session.Session{
		ID:        sessionID,
		UserID:    "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    session.StatusActive,
		Labels:    make(map[string]string),
		Metadata:  make(map[string]interface{}),
	}
	err := store.Create(ctx, sess)
	require.NoError(t, err)

	// Get with lock
	retrievedSess, unlock, err := store.GetWithLock(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, unlock)
	defer unlock()

	assert.Equal(t, sessionID, retrievedSess.ID)
	assert.Equal(t, "user1", retrievedSess.UserID)
}

func TestConcurrentBoltStore_BatchUpdate(t *testing.T) {
	store, cleanup := createTestConcurrentStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions
	sessionIDs := []string{"batch-1", "batch-2", "batch-3"}
	for _, id := range sessionIDs {
		sess := session.Session{
			ID:        id,
			UserID:    "user1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Status:    session.StatusActive,
			Labels:    make(map[string]string),
			Metadata:  map[string]interface{}{"counter": 0},
		}
		err := store.Create(ctx, sess)
		require.NoError(t, err)
	}

	// Prepare batch updates
	updates := make(map[string]func(*session.Session) error)
	for _, id := range sessionIDs {
		sessionID := id // Capture for closure
		updates[sessionID] = func(s *session.Session) error {
			s.Metadata["counter"] = float64(100)
			s.Metadata["batch_updated"] = true
			return nil
		}
	}

	// Execute batch update
	err := store.BatchUpdate(ctx, updates)
	require.NoError(t, err)

	// Verify all sessions were updated
	for _, id := range sessionIDs {
		sess, err := store.Get(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, float64(100), sess.Metadata["counter"])
		assert.Equal(t, true, sess.Metadata["batch_updated"])
	}
}

func TestConcurrentBoltStore_SessionLockContention(t *testing.T) {
	store, cleanup := createTestConcurrentStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "contention-test"

	// Create initial session
	sess := session.Session{
		ID:        sessionID,
		UserID:    "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    session.StatusActive,
		Labels:    make(map[string]string),
		Metadata: map[string]interface{}{
			"operations": []string{},
		},
	}
	err := store.Create(ctx, sess)
	require.NoError(t, err)

	// Run concurrent operations with high contention
	var wg sync.WaitGroup
	goroutines := 20
	operationsPerGoroutine := 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				err := store.UpdateAtomic(ctx, sessionID, func(s *session.Session) error {
					// Handle JSON unmarshaling of arrays
					var ops []string
					if opsInterface, ok := s.Metadata["operations"].([]interface{}); ok {
						for _, op := range opsInterface {
							if str, ok := op.(string); ok {
								ops = append(ops, str)
							}
						}
					} else if opsStr, ok := s.Metadata["operations"].([]string); ok {
						ops = opsStr
					}
					ops = append(ops, fmt.Sprintf("worker-%d-op-%d", workerID, j))
					s.Metadata["operations"] = ops
					return nil
				})
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations were recorded
	finalSess, err := store.Get(ctx, sessionID)
	require.NoError(t, err)
	// Handle JSON unmarshaling of arrays
	var ops []string
	if opsInterface, ok := finalSess.Metadata["operations"].([]interface{}); ok {
		for _, op := range opsInterface {
			if str, ok := op.(string); ok {
				ops = append(ops, str)
			}
		}
	}
	assert.Equal(t, goroutines*operationsPerGoroutine, len(ops))
}

func TestConcurrentBoltStore_DeadlockPrevention(t *testing.T) {
	store, cleanup := createTestConcurrentStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create sessions that will be updated in different orders
	sessions := []string{"deadlock-a", "deadlock-b", "deadlock-c", "deadlock-d"}
	for _, id := range sessions {
		sess := session.Session{
			ID:        id,
			UserID:    "user1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Status:    session.StatusActive,
			Labels:    make(map[string]string),
			Metadata:  map[string]interface{}{"value": 0},
		}
		err := store.Create(ctx, sess)
		require.NoError(t, err)
	}

	// Run concurrent batch updates with different orderings
	var wg sync.WaitGroup

	// Worker 1: Updates A, B, C, D
	wg.Add(1)
	go func() {
		defer wg.Done()
		updates := make(map[string]func(*session.Session) error)
		for _, id := range sessions {
			updates[id] = func(s *session.Session) error {
				var val float64
				if v, ok := s.Metadata["value"].(float64); ok {
					val = v
				}
				s.Metadata["value"] = val + 1
				return nil
			}
		}
		err := store.BatchUpdate(ctx, updates)
		assert.NoError(t, err)
	}()

	// Worker 2: Updates D, C, B, A (reverse order)
	wg.Add(1)
	go func() {
		defer wg.Done()
		updates := make(map[string]func(*session.Session) error)
		for i := len(sessions) - 1; i >= 0; i-- {
			id := sessions[i]
			updates[id] = func(s *session.Session) error {
				var val float64
				if v, ok := s.Metadata["value"].(float64); ok {
					val = v
				}
				s.Metadata["value"] = val + 10
				return nil
			}
		}
		err := store.BatchUpdate(ctx, updates)
		assert.NoError(t, err)
	}()

	// Worker 3: Updates B, D, A, C (mixed order)
	wg.Add(1)
	go func() {
		defer wg.Done()
		mixedOrder := []string{"deadlock-b", "deadlock-d", "deadlock-a", "deadlock-c"}
		updates := make(map[string]func(*session.Session) error)
		for _, id := range mixedOrder {
			updates[id] = func(s *session.Session) error {
				var val float64
				if v, ok := s.Metadata["value"].(float64); ok {
					val = v
				}
				s.Metadata["value"] = val + 100
				return nil
			}
		}
		err := store.BatchUpdate(ctx, updates)
		assert.NoError(t, err)
	}()

	// Should complete without deadlock
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected - batch updates did not complete")
	}

	// Verify all updates were applied
	for _, id := range sessions {
		sess, err := store.Get(ctx, id)
		require.NoError(t, err)
		val := sess.Metadata["value"].(float64)
		assert.Equal(t, float64(111), val, "Each session should have been incremented by 1+10+100")
	}
}
