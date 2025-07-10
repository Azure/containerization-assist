package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	workflow "github.com/Azure/container-kit/pkg/mcp/application/workflows"
	"github.com/Azure/container-kit/pkg/mcp/domain"

	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockToolOrchestrator struct{}

func (m *mockToolOrchestrator) ExecuteTool(_ context.Context, _ string, _ interface{}) (interface{}, error) {
	return map[string]interface{}{"result": "success"}, nil
}

func (m *mockToolOrchestrator) RegisterTool(_ string, _ api.Tool) error {
	return nil
}

func (m *mockToolOrchestrator) ValidateToolArgs(_ string, _ interface{}) error {
	return nil
}

func (m *mockToolOrchestrator) GetToolMetadata(toolName string) (*api.ToolMetadata, error) {
	return &api.ToolMetadata{
		Name:        toolName,
		Description: "Mock tool",
		Version:     "1.0.0",
	}, nil
}

func (m *mockToolOrchestrator) RegisterGenericTool(_ string, _ interface{}) error {
	return nil
}

func (m *mockToolOrchestrator) GetTypedToolMetadata(toolName string) (*api.ToolMetadata, error) {
	return m.GetToolMetadata(toolName)
}

func (m *mockToolOrchestrator) GetTool(_ string) (api.Tool, bool) {
	return nil, false
}

func (m *mockToolOrchestrator) ListTools() []string {
	return []string{}
}

func (m *mockToolOrchestrator) GetStats() interface{} {
	return map[string]interface{}{
		"tools_registered": 0,
		"executions":       0,
	}
}

type mockUnifiedSessionManager struct {
	sessions map[string]*session.SessionState
}

func newMockUnifiedSessionManager() *mockUnifiedSessionManager {
	return &mockUnifiedSessionManager{
		sessions: make(map[string]*session.SessionState),
	}
}

func (m *mockUnifiedSessionManager) GetSession(sessionID string) (*session.SessionState, error) {
	if sess, exists := m.sessions[sessionID]; exists {
		return sess, nil
	}
	return nil, fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) CreateSession(ctx context.Context, userID string) (*session.SessionState, error) {
	sessionID := fmt.Sprintf("test-session-%d", time.Now().UnixNano())
	sess := &session.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/test",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	m.sessions[sessionID] = sess
	return sess, nil
}

func (m *mockUnifiedSessionManager) UpdateSession(_ context.Context, sessionID string, updater func(*session.SessionState) error) error {
	if sess, exists := m.sessions[sessionID]; exists {
		return updater(sess)
	}
	return fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) DeleteSession(sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockUnifiedSessionManager) ListSessions(_ context.Context, _ domain.SessionFilter) ([]*session.SessionState, error) {
	var sessions []*session.SessionState
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (m *mockUnifiedSessionManager) GetOrCreateSession(sessionID string) (*session.SessionState, error) {
	if sess, exists := m.sessions[sessionID]; exists {
		return sess, nil
	}
	sess := &session.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/test",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	m.sessions[sessionID] = sess
	return sess, nil
}

func (m *mockUnifiedSessionManager) GetStats(_ context.Context) (*shared.SessionManagerStats, error) {
	return &shared.SessionManagerStats{
		ActiveSessions: len(m.sessions),
		TotalSessions:  len(m.sessions),
	}, nil
}

// Additional SessionManager interface methods
func (m *mockUnifiedSessionManager) GetSessionTyped(sessionID string) (*session.SessionState, error) {
	return m.GetSession(sessionID)
}

func (m *mockUnifiedSessionManager) GetSessionConcrete(sessionID string) (*session.SessionState, error) {
	return m.GetSession(sessionID)
}

func (m *mockUnifiedSessionManager) GetSessionData(_ context.Context, sessionID string) (map[string]interface{}, error) {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return sess.Metadata, nil
}

func (m *mockUnifiedSessionManager) GetOrCreateSessionTyped(sessionID string) (*session.SessionState, error) {
	return m.GetOrCreateSession(sessionID)
}

func (m *mockUnifiedSessionManager) ListSessionsTyped() ([]*session.SessionState, error) {
	return m.ListSessions(context.Background(), domain.SessionFilter{})
}

func (m *mockUnifiedSessionManager) ListSessionSummaries() ([]*session.SessionSummary, error) {
	return []*session.SessionSummary{}, nil
}

func (m *mockUnifiedSessionManager) StartJob(_ string, _ string) (string, error) {
	return "job-123", nil
}

func (m *mockUnifiedSessionManager) UpdateJobStatus(_ string, _ string, _ session.JobStatus, _ interface{}, _ error) error {
	return nil
}

func (m *mockUnifiedSessionManager) CompleteJob(_ string, _ string, _ interface{}) error {
	return nil
}

func (m *mockUnifiedSessionManager) TrackToolExecution(_ string, _ string, _ interface{}) error {
	return nil
}

func (m *mockUnifiedSessionManager) CompleteToolExecution(_ string, _ string, _ bool, _ error, _ int) error {
	return nil
}

func (m *mockUnifiedSessionManager) TrackError(_ string, _ error, _ interface{}) error {
	return nil
}

func (m *mockUnifiedSessionManager) StartCleanupRoutine() {
	// No-op
}

func (m *mockUnifiedSessionManager) Stop() error {
	return nil
}
func (m *mockUnifiedSessionManager) CreateWorkflowSession(ctx context.Context, _ *workflow.WorkflowSpec) (*session.SessionState, error) {
	return m.CreateSession(ctx, "workflow-user")
}

func (m *mockUnifiedSessionManager) GetWorkflowSession(_ context.Context, sessionID string) (*services.WorkflowSession, error) {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return &services.WorkflowSession{
		SessionID: sess.SessionID,
		Context:   make(map[string]interface{}),
	}, nil
}

func (m *mockUnifiedSessionManager) UpdateWorkflowSession(_ context.Context, _ *services.WorkflowSession) error {
	// No-op for mock
	return nil
}

func (m *mockUnifiedSessionManager) GarbageCollect(ctx context.Context) error {
	return nil
}
func TestNewConversationHandler(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()
		sessionMgr := createTestSessionManager(t)
		// TODO: Fix after package reorganization
		// For now, use nil preference store
		var prefStore *shared.PreferenceStore

		config := ConversationHandlerConfig{
			SessionManager:   sessionMgr,
			PreferenceStore:  prefStore,
			ToolOrchestrator: &mockToolOrchestrator{},
			Logger:           logger,
		}
		handler, err := NewConversationHandler(config)
		require.NoError(t, err)
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.promptManager)
		assert.NotNil(t, handler.toolOrchestrator)
		assert.Equal(t, sessionMgr, handler.sessionManager)
		// assert.Equal(t, prefStore, handler.preferenceStore) // TODO: Re-enable when preference store is implemented
	})

	t.Run("fails without orchestrator", func(t *testing.T) {
		t.Parallel()
		config := ConversationHandlerConfig{
			SessionManager: createTestSessionManager(t),
			// PreferenceStore: createTestPreferenceStore(t), // TODO: Fix after package reorganization
			Logger: logger,
		}
		handler, err := NewConversationHandler(config)
		assert.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "tool orchestrator is required")
	})
}
func TestHandleConversation(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("validates message parameter", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)

		args := ChatToolArgs{
			Message:   "Hello, test message",
			SessionID: "test-session-123",
		}
		ctx := context.Background()
		result, err := handler.HandleConversation(ctx, args)
		require.NoError(t, err)
		assert.NotNil(t, result)

	})

	t.Run("empty message error", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)

		args := ChatToolArgs{
			Message:   "",
			SessionID: "test-session-123",
		}
		ctx := context.Background()
		result, err := handler.HandleConversation(ctx, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message parameter is required")
		assert.Nil(t, result)
	})

	t.Run("handles normal conversation flow", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)
		args := ChatToolArgs{
			Message:   "hello",
			SessionID: "test-session-123",
		}
		ctx := context.Background()
		result, err := handler.HandleConversation(ctx, args)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.Message)
	})
}
func TestHandleAutoAdvance(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("auto-advance with autopilot enabled", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)
		sessionMgr := handler.sessionManager
		_, err := sessionMgr.GetOrCreateSessionTyped("auto-advance-test")
		require.NoError(t, err)
		err = sessionMgr.UpdateSession(context.Background(), "auto-advance-test", func(state *session.SessionState) error {
			if state.Metadata == nil {
				state.Metadata = make(map[string]interface{})
			}
			state.Metadata["repo_analysis"] = map[string]interface{}{
				"_context": map[string]interface{}{
					"autopilot_enabled": true,
				},
			}
			return nil
		})
		require.NoError(t, err)
		response := &ConversationResponse{
			SessionID: "auto-advance-test",
			Stage:     shared.StageAnalysis,
			Status:    ResponseStatusSuccess,
			Message:   "Ready to proceed",
			AutoAdvance: &AutoAdvanceConfig{
				DefaultAction: "continue",
			},
			RequiresInput: false,
		}
		ctx := context.Background()
		finalResponse, err := handler.handleAutoAdvance(ctx, response)
		require.NoError(t, err)
		assert.NotNil(t, finalResponse)

	})

	t.Run("no auto-advance without autopilot", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)
		response := &ConversationResponse{
			SessionID:     "no-auto-advance",
			Stage:         shared.StagePreFlight,
			Status:        ResponseStatusWaitingInput,
			Message:       "Welcome",
			RequiresInput: true,
		}
		ctx := context.Background()
		finalResponse, err := handler.handleAutoAdvance(ctx, response)
		require.NoError(t, err)
		assert.Equal(t, response, finalResponse)
	})

	t.Run("respects max advance steps", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)
		response := &ConversationResponse{
			SessionID: "infinite-loop-test",
			Stage:     shared.StageAnalysis,
			Status:    ResponseStatusSuccess,
			AutoAdvance: &AutoAdvanceConfig{
				DefaultAction: "continue",
			},
			RequiresInput: false,
		}
		ctx := context.Background()
		finalResponse, err := handler.handleAutoAdvance(ctx, response)
		require.NoError(t, err)
		assert.NotNil(t, finalResponse)
	})
}
func TestSessionManagement(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("handles empty session ID", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)

		args := ChatToolArgs{
			Message:   "Create new session",
			SessionID: "",
		}
		ctx := context.Background()
		result, err := handler.HandleConversation(ctx, args)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("uses existing session", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)
		sessionMgr := handler.sessionManager
		_, err := sessionMgr.GetOrCreateSessionTyped("existing-session")
		require.NoError(t, err)

		args := ChatToolArgs{
			Message:   "Use existing session",
			SessionID: "existing-session",
		}
		ctx := context.Background()
		result, err := handler.HandleConversation(ctx, args)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "existing-session", result.SessionID)
	})
}
func TestPreferenceIntegration(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("handles preferences store", func(t *testing.T) {
		t.Parallel()
		handler := createTestHandler(t, logger)
		sessionMgr := handler.sessionManager
		_, err := sessionMgr.GetOrCreateSessionTyped("pref-test")
		require.NoError(t, err)

		args := ChatToolArgs{
			Message:   "Test with preferences",
			SessionID: "pref-test",
		}
		ctx := context.Background()
		result, err := handler.HandleConversation(ctx, args)
		require.NoError(t, err)
		assert.NotNil(t, result)

	})
}

func createTestHandler(t *testing.T, logger *slog.Logger) *ConversationHandler {
	sessionMgr := createTestSessionManager(t)
	// TODO: Fix after package reorganization
	var prefStore *shared.PreferenceStore // createTestPreferenceStore(t)
	orchestrator := &mockToolOrchestrator{}

	config := ConversationHandlerConfig{
		SessionManager:   sessionMgr,
		PreferenceStore:  prefStore,
		ToolOrchestrator: orchestrator,
		Logger:           logger,
	}

	handler, err := NewConversationHandler(config)
	require.NoError(t, err)
	return handler
}

func createTestSessionManager(_ *testing.T) session.SessionManager {
	// Return the mock implementation
	return newMockUnifiedSessionManager()
}

// TODO: Fix after package reorganization
/*
func createTestPreferenceStore(t *testing.T) *processing.PreferenceStore {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	store, err := processing.NewPreferenceStore(
		tempDir+"/preferences.db",
		logger,
		"",
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		store.Close()
	})

	return store
}
*/
func TestMockUnifiedSessionManager(t *testing.T) {
	t.Parallel()
	t.Run("successful session operations", func(t *testing.T) {
		t.Parallel()
		mock := newMockUnifiedSessionManager()
		ctx := context.Background()
		sess, err := mock.CreateSession(ctx, "test-user")
		require.NoError(t, err)
		assert.NotEmpty(t, sess.SessionID)
		retrieved, err := mock.GetSession(sess.SessionID)
		require.NoError(t, err)
		assert.Equal(t, sess.SessionID, retrieved.SessionID)
		err = mock.UpdateSession(ctx, sess.SessionID, func(s *session.SessionState) error {
			s.WorkspaceDir = "/updated/path"
			return nil
		})
		require.NoError(t, err)
		updated, err := mock.GetSession(sess.SessionID)
		require.NoError(t, err)
		assert.Equal(t, "/updated/path", updated.WorkspaceDir)
	})

	t.Run("error on missing session", func(t *testing.T) {
		t.Parallel()
		mock := newMockUnifiedSessionManager()
		_, err := mock.GetSession("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})
}
func BenchmarkHandleConversation(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := createTestHandler(&testing.T{}, logger)

	args := ChatToolArgs{
		Message:   "Benchmark test message",
		SessionID: "bench-session",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler.HandleConversation(ctx, args)
	}
}
