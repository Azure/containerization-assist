package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// setupTestStore creates a temporary BoltDB store for testing
func setupTestStore(t *testing.T) (*BoltSessionStore, func()) {
	tempDir, err := os.MkdirTemp("", "session_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test_sessions.db")
	store, err := NewBoltSessionStore(context.Background(), dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create session store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return store, cleanup
}

// createTestSession creates a test session state
func createTestSession(sessionID string) *SessionState {
	return &SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/workspace",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		StageHistory: []ToolExecution{
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
}

func TestBoltSessionStore_NewStore(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	if store.db == nil {
		t.Error("Database connection should not be nil")
	}
}

func TestBoltSessionStore_SaveAndLoad(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	sessionID := "test-session-123"
	state := createTestSession(sessionID)

	// Test Save
	err := store.Save(context.Background(), sessionID, state)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Test Load
	loadedState, err := store.Load(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	// Verify loaded state
	verifySessionState(t, state, loadedState)
}

func TestBoltSessionStore_List(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Save multiple sessions
	sessions := []string{"session-1", "session-2", "session-3"}
	for _, sessionID := range sessions {
		state := &SessionState{
			SessionID:    sessionID,
			WorkspaceDir: "/tmp/" + sessionID,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
		}
		err := store.Save(context.Background(), sessionID, state)
		if err != nil {
			t.Fatalf("Failed to save session %s: %v", sessionID, err)
		}
	}

	// Test List
	sessionList, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessionList) < len(sessions) {
		t.Errorf("Expected at least %d sessions, got %d", len(sessions), len(sessionList))
	}

	// Check that all our test sessions are in the list
	verifySessionsInList(t, sessions, sessionList)
}

func TestBoltSessionStore_Delete(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	sessionID := "session-to-delete"
	state := &SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/delete-test",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}

	// Save session
	err := store.Save(context.Background(), sessionID, state)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Verify it exists
	_, err = store.Load(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("Session should exist before deletion: %v", err)
	}

	// Delete session
	err = store.Delete(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify it's gone
	_, err = store.Load(context.Background(), sessionID)
	if err == nil {
		t.Error("Session should not exist after deletion")
	}
}

func TestBoltSessionStore_LoadNonExistent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.Load(context.Background(), "non-existent-session")
	if err == nil {
		t.Error("Loading non-existent session should return an error")
	}
}

func TestBoltSessionStore_DeleteNonExistent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	err := store.Delete(context.Background(), "non-existent-session")
	// Deleting non-existent session should not error (idempotent)
	if err != nil {
		t.Errorf("Deleting non-existent session should not error: %v", err)
	}
}

func TestBoltSessionStoreInvalidPath(t *testing.T) {
	// Test with invalid path
	_, err := NewBoltSessionStore(context.Background(), "/invalid/path/that/does/not/exist/test.db")
	if err == nil {
		t.Error("Creating store with invalid path should return an error")
	}
}

// Helper functions to reduce complexity

func verifySessionState(t *testing.T, expected, actual *SessionState) {
	if actual == nil {
		t.Fatal("Loaded state should not be nil")
	}

	if actual.SessionID != expected.SessionID {
		t.Errorf("Expected session ID %s, got %s", expected.SessionID, actual.SessionID)
	}

	if actual.WorkspaceDir != expected.WorkspaceDir {
		t.Errorf("Expected workspace dir %s, got %s", expected.WorkspaceDir, actual.WorkspaceDir)
	}

	// Verify StageHistory is preserved
	if len(actual.StageHistory) != len(expected.StageHistory) {
		t.Errorf("Expected %d stage history entries, got %d", len(expected.StageHistory), len(actual.StageHistory))
	}

	// Verify ScanSummary is preserved
	verifyScanSummary(t, expected.ScanSummary, actual.ScanSummary)
}

func verifyScanSummary(t *testing.T, expected, actual *types.RepositoryScanSummary) {
	if expected == nil && actual == nil {
		return
	}

	if (expected == nil) != (actual == nil) {
		t.Errorf("ScanSummary mismatch: expected %v, got %v", expected, actual)
		return
	}

	if actual.Language != expected.Language {
		t.Errorf("Expected language %s, got %s", expected.Language, actual.Language)
	}
}

func verifySessionsInList(t *testing.T, expected []string, actual []string) {
	sessionMap := make(map[string]bool)
	for _, id := range actual {
		sessionMap[id] = true
	}

	for _, sessionID := range expected {
		if !sessionMap[sessionID] {
			t.Errorf("Session %s not found in list", sessionID)
		}
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
	store, err := NewBoltSessionStore(context.Background(), dbPath)
	if err != nil {
		b.Fatalf("Failed to create session store: %v", err)
	}
	defer store.Close()

	state := createTestSession("benchmark-session")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := "benchmark-session"
		err := store.Save(context.Background(), sessionID, state)
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
	store, err := NewBoltSessionStore(context.Background(), dbPath)
	if err != nil {
		b.Fatalf("Failed to create session store: %v", err)
	}
	defer store.Close()

	sessionID := "benchmark-session"
	state := createTestSession(sessionID)
	err = store.Save(context.Background(), sessionID, state)
	if err != nil {
		b.Fatalf("Failed to save session: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.Load(context.Background(), sessionID)
		if err != nil {
			b.Fatalf("Failed to load session: %v", err)
		}
	}
}
