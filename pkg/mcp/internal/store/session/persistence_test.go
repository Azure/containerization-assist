package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
)

func TestBoltSessionStore(t *testing.T) {
	// Create temporary directory for test database
	tempDir, err := os.MkdirTemp("", "session_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_sessions.db")

	t.Run("NewBoltSessionStore", func(t *testing.T) {
		store, err := NewBoltSessionStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create session store: %v", err)
		}
		defer store.Close()

		if store.db == nil {
			t.Error("Database connection should not be nil")
		}
	})

	t.Run("SaveAndLoad", func(t *testing.T) {
		store, err := NewBoltSessionStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create session store: %v", err)
		}
		defer store.Close()

		// Create test session state
		sessionID := "test-session-123"
		state := &sessiontypes.SessionState{
			SessionID:    sessionID,
			WorkspaceDir: "/tmp/workspace",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			StageHistory: []sessiontypes.ToolExecution{
				{
					Tool:      "analyze_repository",
					StartTime: time.Now(),
					EndTime:   func() *time.Time { t := time.Now(); return &t }(),
					Success:   true,
				},
			},
			ScanSummary: &types.RepositoryScanSummary{
				Language:      "go",
				FilesAnalyzed: 10,
			},
		}

		// Test Save
		err = store.Save(sessionID, state)
		if err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Test Load
		loadedState, err := store.Load(sessionID)
		if err != nil {
			t.Fatalf("Failed to load session: %v", err)
		}

		if loadedState == nil {
			t.Fatal("Loaded state should not be nil")
		}

		if loadedState.SessionID != sessionID {
			t.Errorf("Expected session ID %s, got %s", sessionID, loadedState.SessionID)
		}

		if loadedState.WorkspaceDir != state.WorkspaceDir {
			t.Errorf("Expected workspace dir %s, got %s", state.WorkspaceDir, loadedState.WorkspaceDir)
		}

		// Verify StageHistory is preserved
		if len(loadedState.StageHistory) != len(state.StageHistory) {
			t.Errorf("Expected %d stage history entries, got %d", len(state.StageHistory), len(loadedState.StageHistory))
		}

		// Verify ScanSummary is preserved
		if loadedState.ScanSummary == nil || state.ScanSummary == nil {
			if loadedState.ScanSummary != state.ScanSummary {
				t.Errorf("ScanSummary mismatch: expected %v, got %v", state.ScanSummary, loadedState.ScanSummary)
			}
		} else {
			if loadedState.ScanSummary.Language != state.ScanSummary.Language {
				t.Errorf("Expected language %s, got %s", state.ScanSummary.Language, loadedState.ScanSummary.Language)
			}
		}
	})

	t.Run("List", func(t *testing.T) {
		store, err := NewBoltSessionStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create session store: %v", err)
		}
		defer store.Close()

		// Save multiple sessions
		sessions := []string{"session-1", "session-2", "session-3"}
		for _, sessionID := range sessions {
			state := &sessiontypes.SessionState{
				SessionID:    sessionID,
				WorkspaceDir: "/tmp/" + sessionID,
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
			}
			err = store.Save(sessionID, state)
			if err != nil {
				t.Fatalf("Failed to save session %s: %v", sessionID, err)
			}
		}

		// Test List
		sessionList, err := store.List()
		if err != nil {
			t.Fatalf("Failed to list sessions: %v", err)
		}

		if len(sessionList) < len(sessions) {
			t.Errorf("Expected at least %d sessions, got %d", len(sessions), len(sessionList))
		}

		// Check that all our test sessions are in the list
		sessionMap := make(map[string]bool)
		for _, id := range sessionList {
			sessionMap[id] = true
		}

		for _, sessionID := range sessions {
			if !sessionMap[sessionID] {
				t.Errorf("Session %s not found in list", sessionID)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store, err := NewBoltSessionStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create session store: %v", err)
		}
		defer store.Close()

		sessionID := "session-to-delete"
		state := &sessiontypes.SessionState{
			SessionID:    sessionID,
			WorkspaceDir: "/tmp/delete-test",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
		}

		// Save session
		err = store.Save(sessionID, state)
		if err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Verify it exists
		_, err = store.Load(sessionID)
		if err != nil {
			t.Fatalf("Session should exist before deletion: %v", err)
		}

		// Delete session
		err = store.Delete(sessionID)
		if err != nil {
			t.Fatalf("Failed to delete session: %v", err)
		}

		// Verify it's gone
		_, err = store.Load(sessionID)
		if err == nil {
			t.Error("Session should not exist after deletion")
		}
	})

	t.Run("LoadNonExistent", func(t *testing.T) {
		store, err := NewBoltSessionStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create session store: %v", err)
		}
		defer store.Close()

		_, err = store.Load("non-existent-session")
		if err == nil {
			t.Error("Loading non-existent session should return an error")
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		store, err := NewBoltSessionStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create session store: %v", err)
		}
		defer store.Close()

		err = store.Delete("non-existent-session")
		// Deleting non-existent session should not error (idempotent)
		if err != nil {
			t.Errorf("Deleting non-existent session should not error: %v", err)
		}
	})
}

func TestBoltSessionStoreInvalidPath(t *testing.T) {
	// Test with invalid path
	_, err := NewBoltSessionStore("/invalid/path/that/does/not/exist/test.db")
	if err == nil {
		t.Error("Creating store with invalid path should return an error")
	}
}

// Benchmark basic operations
func BenchmarkBoltSessionStore_Save(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "session_bench_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "bench_sessions.db")
	store, err := NewBoltSessionStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create session store: %v", err)
	}
	defer store.Close()

	state := &sessiontypes.SessionState{
		SessionID:    "benchmark-session",
		WorkspaceDir: "/tmp/benchmark",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := "bench-session-" + string(rune(i))
		state.SessionID = sessionID
		err := store.Save(sessionID, state)
		if err != nil {
			b.Fatalf("Failed to save session: %v", err)
		}
	}
}

func BenchmarkBoltSessionStore_Load(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "session_bench_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "bench_sessions.db")
	store, err := NewBoltSessionStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create session store: %v", err)
	}
	defer store.Close()

	// Pre-populate with test data
	sessionID := "benchmark-load-session"
	state := &sessiontypes.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/benchmark",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}
	err = store.Save(sessionID, state)
	if err != nil {
		b.Fatalf("Failed to save test session: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.Load(sessionID)
		if err != nil {
			b.Fatalf("Failed to load session: %v", err)
		}
	}
}
