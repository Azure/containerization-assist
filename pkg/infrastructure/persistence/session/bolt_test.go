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

	"github.com/Azure/containerization-assist/pkg/domain/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltStore_BasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_sessions.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	store, err := NewBoltStore(dbPath, logger)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Test session creation
	testSession := session.Session{
		ID:        "test-session-1",
		UserID:    "user-123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Status:    session.StatusActive,
		Stage:     "initialization",
		Labels:    map[string]string{"env": "test"},
		Metadata:  map[string]interface{}{"key": "value"},
	}

	// Create
	err = store.Create(ctx, testSession)
	assert.NoError(t, err, "Should create session successfully")

	// Get
	retrieved, err := store.Get(ctx, testSession.ID)
	assert.NoError(t, err, "Should retrieve session successfully")
	assert.Equal(t, testSession.ID, retrieved.ID)
	assert.Equal(t, testSession.UserID, retrieved.UserID)
	assert.Equal(t, testSession.Status, retrieved.Status)

	// Update
	testSession.Status = session.StatusSuspended
	testSession.UpdatedAt = time.Now()
	err = store.Update(ctx, testSession)
	assert.NoError(t, err, "Should update session successfully")

	// Verify update
	updated, err := store.Get(ctx, testSession.ID)
	assert.NoError(t, err, "Should retrieve updated session")
	assert.Equal(t, session.StatusSuspended, updated.Status)

	// Exists
	exists, err := store.Exists(ctx, testSession.ID)
	assert.NoError(t, err, "Should check existence successfully")
	assert.True(t, exists, "Session should exist")

	// Delete
	err = store.Delete(ctx, testSession.ID)
	assert.NoError(t, err, "Should delete session successfully")

	// Verify deletion
	exists, err = store.Exists(ctx, testSession.ID)
	assert.NoError(t, err, "Should check existence after delete")
	assert.False(t, exists, "Session should not exist after deletion")
}

func TestBoltStore_ConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "concurrent_test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise during concurrent tests
	}))

	store, err := NewBoltStore(dbPath, logger)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	numGoroutines := 50
	numOperationsPerGoroutine := 10

	var wg sync.WaitGroup
	var mu sync.Mutex
	createdSessions := make(map[string]bool)
	errors := make([]error, 0)

	// Test concurrent creates
	t.Run("Concurrent Creates", func(t *testing.T) {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < numOperationsPerGoroutine; j++ {
					sessionID := fmt.Sprintf("concurrent-session-%d-%d", goroutineID, j)
					testSession := session.Session{
						ID:        sessionID,
						UserID:    fmt.Sprintf("user-%d", goroutineID),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						ExpiresAt: time.Now().Add(time.Hour),
						Status:    session.StatusActive,
						Stage:     "concurrent-test",
						Labels:    map[string]string{"goroutine": fmt.Sprintf("%d", goroutineID)},
						Metadata:  map[string]interface{}{"iteration": j},
					}

					err := store.Create(ctx, testSession)
					if err != nil {
						mu.Lock()
						errors = append(errors, fmt.Errorf("goroutine %d, iteration %d: %w", goroutineID, j, err))
						mu.Unlock()
					} else {
						mu.Lock()
						createdSessions[sessionID] = true
						mu.Unlock()
					}
				}
			}(i)
		}

		wg.Wait()

		// Check for errors
		if len(errors) > 0 {
			for _, err := range errors {
				t.Errorf("Concurrent create error: %v", err)
			}
		}

		// Verify all sessions were created
		expectedCount := numGoroutines * numOperationsPerGoroutine
		assert.Equal(t, expectedCount, len(createdSessions), "All sessions should be created successfully")
	})

	// Test concurrent reads of the same sessions
	t.Run("Concurrent Reads", func(t *testing.T) {
		// Pick a few session IDs to read concurrently
		sessionIDs := make([]string, 0, 5)
		for id := range createdSessions {
			sessionIDs = append(sessionIDs, id)
			if len(sessionIDs) >= 5 {
				break
			}
		}

		readErrors := make([]error, 0)
		var readMu sync.Mutex

		for _, sessionID := range sessionIDs {
			for i := 0; i < 20; i++ { // 20 concurrent reads per session
				wg.Add(1)
				go func(id string, iteration int) {
					defer wg.Done()

					_, err := store.Get(ctx, id)
					if err != nil {
						readMu.Lock()
						readErrors = append(readErrors, fmt.Errorf("concurrent read %s-%d: %w", id, iteration, err))
						readMu.Unlock()
					}
				}(sessionID, i)
			}
		}

		wg.Wait()

		// Check for read errors
		if len(readErrors) > 0 {
			for _, err := range readErrors {
				t.Errorf("Concurrent read error: %v", err)
			}
		}
	})

	// Test concurrent updates
	t.Run("Concurrent Updates", func(t *testing.T) {
		// Pick a session to update concurrently
		var targetSessionID string
		for id := range createdSessions {
			targetSessionID = id
			break
		}

		updateErrors := make([]error, 0)
		var updateMu sync.Mutex

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()

				// Get current session
				sess, err := store.Get(ctx, targetSessionID)
				if err != nil {
					updateMu.Lock()
					updateErrors = append(updateErrors, fmt.Errorf("get before update %d: %w", iteration, err))
					updateMu.Unlock()
					return
				}

				// Update it
				sess.UpdatedAt = time.Now()
				sess.Metadata = map[string]interface{}{"updated_by": iteration}

				err = store.Update(ctx, sess)
				if err != nil {
					updateMu.Lock()
					updateErrors = append(updateErrors, fmt.Errorf("concurrent update %d: %w", iteration, err))
					updateMu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		// Updates should succeed (BoltDB handles concurrent writes)
		if len(updateErrors) > 0 {
			for _, err := range updateErrors {
				t.Errorf("Concurrent update error: %v", err)
			}
		}
	})

	// Test concurrent list operations
	t.Run("Concurrent List Operations", func(t *testing.T) {
		listErrors := make([]error, 0)
		var listMu sync.Mutex

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()

				sessions, err := store.List(ctx)
				if err != nil {
					listMu.Lock()
					listErrors = append(listErrors, fmt.Errorf("concurrent list %d: %w", iteration, err))
					listMu.Unlock()
					return
				}

				// Should have all the sessions we created
				if len(sessions) < numGoroutines*numOperationsPerGoroutine {
					listMu.Lock()
					listErrors = append(listErrors, fmt.Errorf("list %d: expected at least %d sessions, got %d",
						iteration, numGoroutines*numOperationsPerGoroutine, len(sessions)))
					listMu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		if len(listErrors) > 0 {
			for _, err := range listErrors {
				t.Errorf("Concurrent list error: %v", err)
			}
		}
	})
}

func TestBoltStore_ConcurrentCreateSameID(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "duplicate_test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	store, err := NewBoltStore(dbPath, logger)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	sessionID := "duplicate-session"
	numGoroutines := 10

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	errorCount := 0

	// Try to create the same session from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			testSession := session.Session{
				ID:        sessionID,
				UserID:    fmt.Sprintf("user-%d", goroutineID),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
				Status:    session.StatusActive,
				Stage:     "duplicate-test",
				Labels:    map[string]string{"goroutine": fmt.Sprintf("%d", goroutineID)},
				Metadata:  map[string]interface{}{"goroutine": goroutineID},
			}

			err := store.Create(ctx, testSession)
			mu.Lock()
			if err != nil {
				errorCount++
				// Should get "already exists" error
				assert.Contains(t, err.Error(), "already exists")
			} else {
				successCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Exactly one should succeed, the rest should fail
	assert.Equal(t, 1, successCount, "Exactly one create should succeed")
	assert.Equal(t, numGoroutines-1, errorCount, "All other creates should fail")

	// Verify the session exists
	exists, err := store.Exists(ctx, sessionID)
	assert.NoError(t, err)
	assert.True(t, exists, "Session should exist after concurrent creates")
}

func TestBoltStore_ConcurrentCleanup(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "cleanup_test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	store, err := NewBoltStore(dbPath, logger)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Create sessions with different expiration times
	activeSessions := 5
	expiredSessions := 5

	// Create active sessions
	for i := 0; i < activeSessions; i++ {
		sess := session.Session{
			ID:        fmt.Sprintf("active-session-%d", i),
			UserID:    "test-user",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour), // Active
			Status:    session.StatusActive,
			Stage:     "test",
		}
		err := store.Create(ctx, sess)
		require.NoError(t, err)
	}

	// Create expired sessions
	for i := 0; i < expiredSessions; i++ {
		sess := session.Session{
			ID:        fmt.Sprintf("expired-session-%d", i),
			UserID:    "test-user",
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
			ExpiresAt: time.Now().Add(-time.Hour), // Expired
			Status:    session.StatusActive,
			Stage:     "test",
		}
		err := store.Create(ctx, sess)
		require.NoError(t, err)
	}

	// Run concurrent cleanup operations
	var wg sync.WaitGroup
	var mu sync.Mutex
	totalCleaned := 0
	cleanupErrors := make([]error, 0)

	numCleanupGoroutines := 5
	for i := 0; i < numCleanupGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			cleaned, err := store.Cleanup(ctx)
			mu.Lock()
			if err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("cleanup goroutine %d: %w", goroutineID, err))
			} else {
				totalCleaned += cleaned
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Check for cleanup errors
	if len(cleanupErrors) > 0 {
		for _, err := range cleanupErrors {
			t.Errorf("Cleanup error: %v", err)
		}
	}

	// Verify that expired sessions were cleaned up
	// Note: totalCleaned might be higher than expiredSessions due to race conditions
	// where multiple goroutines might clean the same sessions
	assert.GreaterOrEqual(t, totalCleaned, expiredSessions, "Should clean up at least the expired sessions")

	// Verify active sessions still exist
	for i := 0; i < activeSessions; i++ {
		exists, err := store.Exists(ctx, fmt.Sprintf("active-session-%d", i))
		assert.NoError(t, err)
		assert.True(t, exists, "Active session should still exist after cleanup")
	}

	// Verify expired sessions are gone
	for i := 0; i < expiredSessions; i++ {
		exists, err := store.Exists(ctx, fmt.Sprintf("expired-session-%d", i))
		assert.NoError(t, err)
		assert.False(t, exists, "Expired session should be cleaned up")
	}
}

func TestBoltStore_StatsUnderConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "stats_test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	store, err := NewBoltStore(dbPath, logger)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Create some sessions
	numSessions := 20
	for i := 0; i < numSessions; i++ {
		status := session.StatusActive
		if i%4 == 0 { // Make some inactive
			status = session.StatusSuspended
		}

		sess := session.Session{
			ID:        fmt.Sprintf("stats-session-%d", i),
			UserID:    "test-user",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
			Status:    status,
			Stage:     "test",
		}
		err := store.Create(ctx, sess)
		require.NoError(t, err)
	}

	// Run concurrent stats operations
	var wg sync.WaitGroup
	var mu sync.Mutex
	statsResults := make([]session.Stats, 0)
	statsErrors := make([]error, 0)

	numStatsGoroutines := 10
	for i := 0; i < numStatsGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			stats, err := store.Stats(ctx)
			mu.Lock()
			if err != nil {
				statsErrors = append(statsErrors, fmt.Errorf("stats goroutine %d: %w", goroutineID, err))
			} else {
				statsResults = append(statsResults, stats)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Check for stats errors
	if len(statsErrors) > 0 {
		for _, err := range statsErrors {
			t.Errorf("Stats error: %v", err)
		}
	}

	// All stats should be consistent
	assert.Equal(t, numStatsGoroutines, len(statsResults), "All stats operations should succeed")

	for _, stats := range statsResults {
		assert.Equal(t, numSessions, stats.TotalSessions, "Total sessions should be consistent")
		assert.Greater(t, stats.ActiveSessions, 0, "Should have some active sessions")
		assert.Less(t, stats.ActiveSessions, numSessions, "Should have some inactive sessions")
	}
}

func TestBoltStore_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "error_test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	store, err := NewBoltStore(dbPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test operations on non-existent sessions
	_, err = store.Get(ctx, "non-existent")
	assert.Error(t, err, "Should error when getting non-existent session")
	assert.Contains(t, err.Error(), "not found")

	err = store.Update(ctx, session.Session{ID: "non-existent"})
	assert.Error(t, err, "Should error when updating non-existent session")
	assert.Contains(t, err.Error(), "not found")

	err = store.Delete(ctx, "non-existent")
	assert.Error(t, err, "Should error when deleting non-existent session")
	assert.Contains(t, err.Error(), "not found")

	// Close the store and test operations on closed store
	_ = store.Close()

	err = store.Create(ctx, session.Session{ID: "test"})
	assert.Error(t, err, "Should error when creating on closed store")

	_, err = store.Get(ctx, "test")
	assert.Error(t, err, "Should error when getting from closed store")
}
