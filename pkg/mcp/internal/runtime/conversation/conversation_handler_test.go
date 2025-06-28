package conversation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/conversation"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSessionManagerAdapter implements orchestration.SessionManager interface
type testSessionManagerAdapter struct {
	mgr *session.SessionManager
}

func (a *testSessionManagerAdapter) GetSession(sessionID string) (interface{}, error) {
	return a.mgr.GetSession(sessionID)
}

func (a *testSessionManagerAdapter) UpdateSession(session interface{}) error {
	s, ok := session.(*sessiontypes.SessionState)
	if !ok {
		return fmt.Errorf("invalid session type: expected *sessiontypes.SessionState, got %T", session)
	}
	if s.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	return a.mgr.UpdateSession(s.SessionID, func(existing interface{}) {
		if state, ok := existing.(*sessiontypes.SessionState); ok {
			*state = *s
		}
	})
}

type mockOrchestrator struct {
	executeCalls []executeCall
	executeFunc  func(ctx context.Context, toolName string, params map[string]interface{}, session interface{}) (interface{}, error)
}

type executeCall struct {
	toolName string
	params   map[string]interface{}
	session  interface{}
}

func (m *mockOrchestrator) ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}, session interface{}) (interface{}, error) {
	m.executeCalls = append(m.executeCalls, executeCall{
		toolName: toolName,
		params:   params,
		session:  session,
	})
	if m.executeFunc != nil {
		return m.executeFunc(ctx, toolName, params, session)
	}
	return map[string]interface{}{"result": "success"}, nil
}

func TestNewConversationHandler(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	t.Run("successful creation", func(t *testing.T) {
		sessionMgr := createTestSessionManager(t)
		prefStore := createTestPreferenceStore(t)

		config := ConversationHandlerConfig{
			SessionManager:   sessionMgr,
			PreferenceStore:  prefStore,
			ToolOrchestrator: &orchestration.MCPToolOrchestrator{},
			Logger:           logger,
		}

		handler, err := NewConversationHandler(config)

		require.NoError(t, err)
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.promptManager)
		assert.NotNil(t, handler.toolOrchestrator)
		assert.Equal(t, sessionMgr, handler.sessionManager)
		assert.Equal(t, prefStore, handler.preferenceStore)
	})

	t.Run("fails without orchestrator", func(t *testing.T) {
		config := ConversationHandlerConfig{
			SessionManager:  createTestSessionManager(t),
			PreferenceStore: createTestPreferenceStore(t),
			Logger:          logger,
		}

		handler, err := NewConversationHandler(config)

		assert.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "tool orchestrator is required")
	})
}

func TestHandleConversation(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	t.Run("validates message parameter", func(t *testing.T) {
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
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	t.Run("auto-advance with autopilot enabled", func(t *testing.T) {
		handler := createTestHandler(t, logger)
		sessionMgr := handler.sessionManager

		_, err := sessionMgr.GetOrCreateSession("auto-advance-test")
		require.NoError(t, err)

		err = sessionMgr.UpdateSession("auto-advance-test", func(s interface{}) {
			if state, ok := s.(*sessiontypes.SessionState); ok {
				state.RepoAnalysis = map[string]interface{}{
					"_context": map[string]interface{}{
						"autopilot_enabled": true,
					},
				}
			}
		})
		require.NoError(t, err)

		response := &ConversationResponse{
			SessionID: "auto-advance-test",
			Stage:     types.StageAnalysis,
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
		handler := createTestHandler(t, logger)

		response := &ConversationResponse{
			SessionID:     "no-auto-advance",
			Stage:         types.StageWelcome,
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
		handler := createTestHandler(t, logger)

		response := &ConversationResponse{
			SessionID: "infinite-loop-test",
			Stage:     types.StageAnalysis,
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
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	t.Run("handles empty session ID", func(t *testing.T) {
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
		handler := createTestHandler(t, logger)
		sessionMgr := handler.sessionManager

		// Create a session
		_, err := sessionMgr.GetOrCreateSession("existing-session")
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
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	t.Run("handles preferences store", func(t *testing.T) {
		handler := createTestHandler(t, logger)

		// Create session
		sessionMgr := handler.sessionManager
		_, err := sessionMgr.GetOrCreateSession("pref-test")
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

func createTestHandler(t *testing.T, logger zerolog.Logger) *ConversationHandler {
	sessionMgr := createTestSessionManager(t)
	prefStore := createTestPreferenceStore(t)

	sessionAdapter := &testSessionManagerAdapter{mgr: sessionMgr}

	registry := orchestration.NewMCPToolRegistry(logger)
	orchestrator := orchestration.NewMCPToolOrchestrator(
		registry,
		sessionAdapter,
		logger,
	)

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
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	mgr, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tempDir,
		StorePath:         tempDir + "/sessions.db",
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

func createTestPreferenceStore(t *testing.T) *utils.PreferenceStore {
	tempDir := t.TempDir()
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	store, err := utils.NewPreferenceStore(
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

func TestSessionManagerAdapter(t *testing.T) {
	t.Run("successful session update", func(t *testing.T) {
		sessionMgr := createTestSessionManager(t)
		adapter := &testSessionManagerAdapter{mgr: sessionMgr}

		_, err := sessionMgr.GetOrCreateSession("update-test")
		require.NoError(t, err)

		updatedSession := &sessiontypes.SessionState{
			SessionID: "update-test",
			ImageRef: types.ImageReference{
				Repository: "test/repo",
				Tag:        "updated",
			},
		}

		err = adapter.UpdateSession(updatedSession)

		require.NoError(t, err)

		retrievedInterface, err := sessionMgr.GetSession("update-test")
		require.NoError(t, err)
		retrieved, ok := retrievedInterface.(*sessiontypes.SessionState)
		require.True(t, ok)
		assert.Equal(t, "test/repo", retrieved.ImageRef.Repository)
		assert.Equal(t, "updated", retrieved.ImageRef.Tag)
	})

	t.Run("error on invalid type", func(t *testing.T) {
		sessionMgr := createTestSessionManager(t)
		adapter := &testSessionManagerAdapter{mgr: sessionMgr}

		err := adapter.UpdateSession("not a session")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid session type")
	})

	t.Run("error on missing session ID", func(t *testing.T) {
		sessionMgr := createTestSessionManager(t)
		adapter := &testSessionManagerAdapter{mgr: sessionMgr}

		session := &sessiontypes.SessionState{
			SessionID: "",
		}

		err := adapter.UpdateSession(session)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session ID is required")
	})
}

func BenchmarkHandleConversation(b *testing.B) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
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
