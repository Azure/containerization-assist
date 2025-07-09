package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"

	// "github.com/Azure/container-kit/pkg/mcp/domain/processing" // TODO: Fix import after package reorganization
	"github.com/Azure/container-kit/pkg/mcp/application/internal/conversation"
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

type mockUnifiedSessionManager struct {
	sessions map[string]*session.SessionState
}

func newMockUnifiedSessionManager() *mockUnifiedSessionManager {
	return &mockUnifiedSessionManager{
		sessions: make(map[string]*session.SessionState),
	}
}

func (m *mockUnifiedSessionManager) GetSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
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
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	m.sessions[sessionID] = sess
	return sess, nil
}

func (m *mockUnifiedSessionManager) UpdateSession(ctx context.Context, sessionID string, updater func(*session.SessionState)) error {
	if sess, exists := m.sessions[sessionID]; exists {
		updater(sess)
		return nil
	}
	return fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockUnifiedSessionManager) ListSessions(ctx context.Context, filter core.SessionFilter) ([]*session.SessionState, error) {
	var sessions []*session.SessionState
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (m *mockUnifiedSessionManager) GetOrCreateSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	if sess, exists := m.sessions[sessionID]; exists {
		return sess, nil
	}
	sess := &session.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/test",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	m.sessions[sessionID] = sess
	return sess, nil
}

func (m *mockUnifiedSessionManager) GetStats(ctx context.Context) (*core.SessionManagerStats, error) {
	return &core.SessionManagerStats{
		ActiveSessions: len(m.sessions),
		TotalSessions:  len(m.sessions),
	}, nil
}
func (m *mockUnifiedSessionManager) CreateWorkflowSession(ctx context.Context, spec *session.WorkflowSpec) (*session.SessionState, error) {
	return m.CreateSession(ctx, "workflow-user")
}

func (m *mockUnifiedSessionManager) GetWorkflowSession(ctx context.Context, sessionID string) (*session.WorkflowSession, error) {
	sess, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return &session.WorkflowSession{SessionState: sess}, nil
}

func (m *mockUnifiedSessionManager) UpdateWorkflowSession(ctx context.Context, workflowSession *session.WorkflowSession) error {
	return m.UpdateSession(ctx, workflowSession.SessionID, func(sess *session.SessionState) {
		*sess = *workflowSession.SessionState
	})
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
		var prefStore *utils.PreferenceStore = nil

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

		args := conversation.ChatToolArgs{
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

		args := conversation.ChatToolArgs{
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
		args := conversation.ChatToolArgs{
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
			Stage:     core.ConversationStageAnalyze,
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
			Stage:         core.ConversationStagePreFlight,
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
			Stage:     core.ConversationStageAnalyze,
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

		args := conversation.ChatToolArgs{
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

		args := conversation.ChatToolArgs{
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

		args := conversation.ChatToolArgs{
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
	var prefStore *utils.PreferenceStore = nil // createTestPreferenceStore(t)
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

func createTestSessionManager(t *testing.T) *session.SessionManager {
	tempDir := t.TempDir()
	logger := slog.Default()

	mgr, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tempDir,
		StorePath:         "",
		SessionTTL:        24 * time.Hour,
		MaxSessions:       10,
		MaxDiskPerSession: 100 * 1024 * 1024,
		TotalDiskLimit:    1024 * 1024 * 1024,
		Logger:            logger,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		mgr.Stop()
	})

	return mgr
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
		retrieved, err := mock.GetSession(ctx, sess.SessionID)
		require.NoError(t, err)
		assert.Equal(t, sess.SessionID, retrieved.SessionID)
		err = mock.UpdateSession(ctx, sess.SessionID, func(s *session.SessionState) {
			s.WorkspaceDir = "/updated/path"
		})
		require.NoError(t, err)
		updated, err := mock.GetSession(ctx, sess.SessionID)
		require.NoError(t, err)
		assert.Equal(t, "/updated/path", updated.WorkspaceDir)
	})

	t.Run("error on missing session", func(t *testing.T) {
		t.Parallel()
		mock := newMockUnifiedSessionManager()
		ctx := context.Background()
		_, err := mock.GetSession(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})
}
func BenchmarkHandleConversation(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := createTestHandler(&testing.T{}, logger)

	args := conversation.ChatToolArgs{
		Message:   "Benchmark test message",
		SessionID: "bench-session",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler.HandleConversation(ctx, args)
	}
}
