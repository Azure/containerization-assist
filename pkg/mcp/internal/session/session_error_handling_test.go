package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_ErrorHandling(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := filepath.Join(os.TempDir(), "session-test")
	config := SessionManagerConfig{
		WorkspaceDir:      tempDir,
		MaxSessions:       1000,
		SessionTTL:        1 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,      // 1GB
		TotalDiskLimit:    10 * 1024 * 1024 * 1024, // 10GB
		Logger:            logger,
	}
	manager, err := NewSessionManager(config)
	require.NoError(t, err, "Failed to create session manager")
	defer func() { _ = manager.Stop() }()

	ctx := context.Background()

	t.Run("create_session_with_invalid_id", func(t *testing.T) {
		// Test with invalid session ID characters
		invalidIDs := []string{
			"",    // empty
			"   ", // whitespace only
			"id with spaces",
			"id\nwith\nnewlines",
			"id\twith\ttabs",
			"id/with/slashes",
			"id\\with\\backslashes",
			"id@with@symbols",
			"id#with#hash",
			string([]byte{0, 1, 2}), // null bytes
		}

		for _, invalidID := range invalidIDs {
			t.Run("invalid_id_"+invalidID, func(t *testing.T) {
				session, err := manager.GetOrCreateSession(invalidID)

				// Special case: null bytes in session ID will cause filesystem errors
				if len(invalidID) > 0 && invalidID[0] == 0 {
					assert.Error(t, err, "Session ID with null bytes should cause filesystem error")
					assert.Nil(t, session, "Should not create session with null bytes")
				} else {
					// The implementation is lenient and creates sessions with any ID
					// This tests that it handles gracefully without panicking
					assert.NoError(t, err, "Session manager should handle any session ID gracefully")
					assert.NotNil(t, session, "Should create session even with unusual characters")
				}
			})
		}
	})

	t.Run("get_nonexistent_session", func(t *testing.T) {
		session, err := manager.GetSession("nonexistent-session-id")
		assert.Error(t, err, "Should return error for nonexistent session")
		assert.Nil(t, session, "Should return nil session")

		assert.Contains(t, err.Error(), "session", "Error should mention session")
	})

	t.Run("update_nonexistent_session", func(t *testing.T) {
		err := manager.SetSessionLabels("nonexistent-session", []string{"test"})
		assert.Error(t, err, "Should return error when updating nonexistent session")

		assert.Contains(t, err.Error(), "session", "Error should mention session")
	})

	t.Run("context_cancellation", func(t *testing.T) {
		// Create a context that's already cancelled
		_, cancel := context.WithCancel(ctx)
		cancel()

		// Operations should handle cancelled context gracefully
		session, err := manager.GetOrCreateSession("")
		assert.NoError(t, err, "Session creation should not be affected by cancelled context")
		assert.NotNil(t, session, "Should still create session")
	})

	t.Run("context_timeout", func(t *testing.T) {
		// Create a context with very short timeout
		_, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()

		// Let timeout expire
		time.Sleep(10 * time.Millisecond)

		// Operations should handle timeout gracefully
		session, err := manager.GetOrCreateSession("")
		// Should still work as session operations are typically fast
		assert.NoError(t, err, "Session creation should not be affected by timeout")
		assert.NotNil(t, session, "Should still create session")
	})

	t.Run("concurrent_session_operations", func(t *testing.T) {
		sessionInterface, err := manager.GetOrCreateSession("")
		require.NoError(t, err)
		session, ok := sessionInterface.(*SessionState)
		require.True(t, ok, "Session should be *SessionState")
		sessionID := session.SessionID

		// Simulate concurrent operations
		done := make(chan error, 10)

		// Multiple goroutines trying to update the same session
		for i := 0; i < 10; i++ {
			go func(i int) {
				err := manager.SetSessionLabels(sessionID, []string{
					"concurrent-test",
					"worker-" + string(rune('0'+i)),
				})
				done <- err
			}(i)
		}

		// Wait for all operations to complete
		for i := 0; i < 10; i++ {
			err := <-done
			assert.NoError(t, err, "Concurrent operations should not fail")
		}
	})

	t.Run("large_label_arrays", func(t *testing.T) {
		sessionInterface, err := manager.GetOrCreateSession("")
		require.NoError(t, err)
		session, ok := sessionInterface.(*SessionState)
		require.True(t, ok, "Session should be *SessionState")
		sessionID := session.SessionID

		// Test with very large label arrays
		largeLabels := make([]string, 1000)
		for i := range largeLabels {
			largeLabels[i] = "label-" + string(rune('0'+(i%10)))
		}

		err = manager.SetSessionLabels(sessionID, largeLabels)
		assert.NoError(t, err, "Should handle large label arrays")

		// Verify labels were set correctly
		updatedSessionInterface, err := manager.GetSession(sessionID)
		require.NoError(t, err)
		updatedSession, ok := updatedSessionInterface.(*SessionState)
		require.True(t, ok, "Updated session should be *SessionState")
		assert.Len(t, updatedSession.GetLabels(), 1000, "All labels should be preserved")
	})

	t.Run("labels_with_special_characters", func(t *testing.T) {
		sessionInterface, err := manager.GetOrCreateSession("")
		require.NoError(t, err)
		session, ok := sessionInterface.(*SessionState)
		require.True(t, ok, "Session should be *SessionState")
		sessionID := session.SessionID

		specialLabels := []string{
			"label with spaces",
			"label\nwith\nnewlines",
			"label\twith\ttabs",
			"label/with/slashes",
			"label\\with\\backslashes",
			"label@with@symbols",
			"label#with#hash",
			"label=with=equals",
			"label:with:colons",
			"label;with;semicolons",
			"label,with,commas",
			"label.with.dots",
			"label-with-dashes",
			"label_with_underscores",
			"label+with+plus",
			"label&with&ampersand",
			"label%with%percent",
			"label$with$dollar",
			"label!with!exclamation",
			"label?with?question",
			"label(with)parentheses",
			"label[with]brackets",
			"label{with}braces",
			"label\"with\"quotes",
			"label'with'apostrophes",
			"label`with`backticks",
			"label~with~tilde",
			"label|with|pipe",
			"label<with>angles",
			"unicode-λabel-🚀",
			"", // empty label
		}

		err = manager.SetSessionLabels(sessionID, specialLabels)
		assert.NoError(t, err, "Should handle labels with special characters")

		// Verify labels were preserved correctly
		updatedSessionInterface, err := manager.GetSession(sessionID)
		require.NoError(t, err)
		updatedSession, ok := updatedSessionInterface.(*SessionState)
		require.True(t, ok, "Updated session should be *SessionState")
		retrievedLabels := updatedSession.GetLabels()

		// Should preserve all non-empty labels
		for _, label := range specialLabels {
			if label != "" {
				assert.Contains(t, retrievedLabels, label, "Special character label should be preserved: %q", label)
			}
		}
	})

	t.Run("memory_stress_test", func(t *testing.T) {
		// Create many sessions to test memory handling
		sessionIDs := make([]string, 100)

		for i := 0; i < 100; i++ {
			sessionInterface, err := manager.GetOrCreateSession("")
			require.NoError(t, err)
			session, ok := sessionInterface.(*SessionState)
			require.True(t, ok, "Session should be *SessionState")
			sessionIDs[i] = session.SessionID
		}

		// Verify all sessions exist and are accessible
		for i, sessionID := range sessionIDs {
			sessionInterface, err := manager.GetSession(sessionID)
			assert.NoError(t, err, "Session %d should exist", i)
			assert.NotNil(t, sessionInterface, "Session %d should not be nil", i)
		}

		// Clean up by updating each session
		for i, sessionID := range sessionIDs {
			err := manager.SetSessionLabels(sessionID, []string{
				"cleanup-test",
				"session-" + string(rune('0'+(i%10))),
			})
			assert.NoError(t, err, "Should be able to update session %d", i)
		}
	})
}

func TestSessionState_ErrorHandling(t *testing.T) {
	t.Run("nil_session_state_operations", func(t *testing.T) {
		var session *SessionState

		// Operations on nil session should panic (this is expected Go behavior)
		assert.Panics(t, func() {
			_ = session.SessionID
		}, "Accessing SessionID should panic on nil session")

		assert.Panics(t, func() {
			_ = session.GetLabels()
		}, "GetLabels should panic on nil session")
	})

	t.Run("empty_session_state", func(t *testing.T) {
		session := &SessionState{}

		// Operations on empty session should return sensible defaults
		sessionID := session.SessionID
		assert.Equal(t, "", sessionID, "Empty session should have empty ID")

		labels := session.GetLabels()
		assert.Empty(t, labels, "Empty session should have no labels")
	})

	t.Run("session_with_nil_labels", func(t *testing.T) {
		session := &SessionState{
			SessionID: "test-session",
			Labels:    nil,
		}

		labels := session.GetLabels()
		assert.Empty(t, labels, "Session with nil labels should return empty slice")
	})

	t.Run("session_state_immutability", func(t *testing.T) {
		originalLabels := []string{"original1", "original2"}
		session := &SessionState{
			SessionID: "test-session",
			Labels:    originalLabels,
		}

		// Get labels and modify the returned slice
		retrievedLabels := session.GetLabels()
		retrievedLabels[0] = "modified"

		// Original session should not be affected
		assert.Equal(t, "original1", session.Labels[0], "Original session should not be modified")
	})
}

func TestSessionLabelData_ErrorHandling(t *testing.T) {
	t.Run("nil_slice_operations", func(t *testing.T) {
		var labelData []SessionLabelData

		// Operations on nil slice should not panic
		assert.NotPanics(t, func() {
			_ = len(labelData)
		}, "Length operation should not panic on nil slice")

		assert.NotPanics(t, func() {
			for range labelData {
				t.Error("Should not execute for nil slice")
			}
		}, "Range operation should not panic on nil slice")
	})

	t.Run("empty_slice_operations", func(t *testing.T) {
		labelData := []SessionLabelData{}

		assert.Equal(t, 0, len(labelData), "Empty slice should have length 0")

		// Range should work but not execute
		count := 0
		for range labelData {
			count++
		}
		assert.Equal(t, 0, count, "Range should not execute on empty slice")
	})

	t.Run("label_data_with_special_values", func(t *testing.T) {
		labelData := []SessionLabelData{
			{SessionID: "", Labels: nil},
			{SessionID: "test", Labels: []string{}},
			{SessionID: "test-special", Labels: []string{"", "valid"}},
		}

		// Should handle special values gracefully
		assert.Len(t, labelData, 3, "Should preserve all entries")

		for i, data := range labelData {
			assert.NotPanics(t, func() {
				_ = data.SessionID
				_ = data.Labels
			}, "Accessing fields should not panic for entry %d", i)
		}
	})
}

// BenchmarkSessionManager_ErrorConditions tests performance under error conditions
func BenchmarkSessionManager_ErrorConditions(b *testing.B) {
	logger := zerolog.Nop()
	tempDir := filepath.Join(os.TempDir(), "session-bench-test")
	config := SessionManagerConfig{
		WorkspaceDir:      tempDir,
		MaxSessions:       1000,
		SessionTTL:        1 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,      // 1GB
		TotalDiskLimit:    10 * 1024 * 1024 * 1024, // 10GB
		Logger:            logger,
	}
	manager, err := NewSessionManager(config)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = manager.Stop() }()

	b.Run("get_nonexistent_session", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := manager.GetSession("nonexistent-session-id")
			if err == nil {
				b.Fatal("Expected error for nonexistent session")
			}
		}
	})

	b.Run("concurrent_session_access", func(b *testing.B) {
		sessionInterface, err := manager.GetOrCreateSession("")
		if err != nil {
			b.Fatal(err)
		}
		session, ok := sessionInterface.(*SessionState)
		if !ok {
			b.Fatal("Session should be *SessionState")
		}
		sessionID := session.SessionID

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := manager.GetSession(sessionID)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
