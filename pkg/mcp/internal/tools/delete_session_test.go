package tools

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSessionManager for testing
type MockSessionManager struct {
	sessions map[string]*SessionData
}

func NewMockSessionManager() *MockSessionManager {
	return &MockSessionManager{
		sessions: make(map[string]*SessionData),
	}
}

func (m *MockSessionManager) GetSession(sessionID string) (*SessionData, error) {
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, nil
	}
	return session, nil
}

func (m *MockSessionManager) DeleteSession(sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *MockSessionManager) CancelSessionJobs(sessionID string) ([]string, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	return session.ActiveJobs, nil
}

func (m *MockSessionManager) GetAllSessions() ([]*SessionData, error) {
	sessions := make([]*SessionData, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (m *MockSessionManager) GetStats() *SessionManagerStats {
	return &SessionManagerStats{
		TotalSessions: len(m.sessions),
	}
}

// MockWorkspaceManager for testing
type MockWorkspaceManager struct {
	workspaces map[string]int64 // sessionID -> size
}

func NewMockWorkspaceManager() *MockWorkspaceManager {
	return &MockWorkspaceManager{
		workspaces: make(map[string]int64),
	}
}

func (m *MockWorkspaceManager) GetWorkspacePath(sessionID string) string {
	return "/tmp/test-workspace/" + sessionID
}

func (m *MockWorkspaceManager) DeleteWorkspace(sessionID string) error {
	delete(m.workspaces, sessionID)
	return nil
}

func (m *MockWorkspaceManager) GetWorkspaceSize(sessionID string) (int64, error) {
	size, exists := m.workspaces[sessionID]
	if !exists {
		return 0, nil
	}
	return size, nil
}

func TestDeleteSessionTool_Execute(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("delete existing session", func(t *testing.T) {
		// Setup
		sessionManager := NewMockSessionManager()
		workspaceManager := NewMockWorkspaceManager()

		sessionID := "test-session-123"
		sessionManager.sessions[sessionID] = &SessionData{
			ID:         sessionID,
			ActiveJobs: []string{},
		}
		workspaceManager.workspaces[sessionID] = 1024 * 1024 // 1MB

		tool := NewDeleteSessionTool(logger, sessionManager, workspaceManager)

		// Execute
		args := DeleteSessionArgs{
			SessionID:       sessionID,
			DeleteWorkspace: true,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Deleted)
		assert.True(t, result.WorkspaceDeleted)
		assert.Equal(t, int64(1024*1024), result.DiskReclaimed)
		assert.Equal(t, sessionID, result.SessionID)

		// Verify session was deleted
		_, exists := sessionManager.sessions[sessionID]
		assert.False(t, exists)
	})

	t.Run("delete non-existent session", func(t *testing.T) {
		// Setup
		sessionManager := NewMockSessionManager()
		workspaceManager := NewMockWorkspaceManager()
		tool := NewDeleteSessionTool(logger, sessionManager, workspaceManager)

		// Execute
		args := DeleteSessionArgs{
			SessionID: "non-existent",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Deleted)
		assert.NotNil(t, result.Error)
		assert.Equal(t, "SESSION_NOT_FOUND", result.Error.Type)
	})

	t.Run("delete session with active jobs without force", func(t *testing.T) {
		// Setup
		sessionManager := NewMockSessionManager()
		workspaceManager := NewMockWorkspaceManager()

		sessionID := "test-session-456"
		sessionManager.sessions[sessionID] = &SessionData{
			ID:         sessionID,
			ActiveJobs: []string{"job1", "job2"},
		}

		tool := NewDeleteSessionTool(logger, sessionManager, workspaceManager)

		// Execute
		args := DeleteSessionArgs{
			SessionID: sessionID,
			Force:     false,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Deleted)
		assert.NotNil(t, result.Error)
		assert.Equal(t, "ACTIVE_JOBS", result.Error.Type)

		// Verify session was not deleted
		_, exists := sessionManager.sessions[sessionID]
		assert.True(t, exists)
	})

	t.Run("force delete session with active jobs", func(t *testing.T) {
		// Setup
		sessionManager := NewMockSessionManager()
		workspaceManager := NewMockWorkspaceManager()

		sessionID := "test-session-789"
		sessionManager.sessions[sessionID] = &SessionData{
			ID:         sessionID,
			ActiveJobs: []string{"job1", "job2"},
		}

		tool := NewDeleteSessionTool(logger, sessionManager, workspaceManager)

		// Execute
		args := DeleteSessionArgs{
			SessionID: sessionID,
			Force:     true,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Deleted)
		assert.Equal(t, []string{"job1", "job2"}, result.JobsCancelled)

		// Verify session was deleted
		_, exists := sessionManager.sessions[sessionID]
		assert.False(t, exists)
	})

	t.Run("empty session ID", func(t *testing.T) {
		// Setup
		sessionManager := NewMockSessionManager()
		workspaceManager := NewMockWorkspaceManager()
		tool := NewDeleteSessionTool(logger, sessionManager, workspaceManager)

		// Execute
		args := DeleteSessionArgs{
			SessionID: "",
		}
		_, err := tool.Execute(context.Background(), args)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session_id is required")
	})
}
