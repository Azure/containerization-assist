package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/conversation"

	runtimeconv "github.com/Azure/container-kit/pkg/mcp/internal/runtime/conversation"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"

	mcp "github.com/Azure/container-kit/pkg/mcp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConversationFlow tests the complete conversation workflow
func TestConversationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "conversation-flow-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create server with conversation mode
	config := ServerConfig{
		WorkspaceDir:      tmpDir,
		MaxSessions:       10,
		SessionTTL:        1 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,
		TotalDiskLimit:    10 * 1024 * 1024 * 1024,
		StorePath:         filepath.Join(tmpDir, "sessions.db"),
		TransportType:     "stdio",
		LogLevel:          "error",
	}

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)
	defer server.Stop()

	// Enable conversation mode
	convConfig := ConversationConfig{
		EnableTelemetry:   false,
		PreferencesDBPath: filepath.Join(tmpDir, "preferences.db"),
	}

	err = server.EnableConversationMode(convConfig)
	require.NoError(t, err)

	adapterInterface := server.GetConversationAdapter()
	require.NotNil(t, adapterInterface)

	// Type assert to the concrete conversation handler type
	adapter, ok := adapterInterface.(interface {
		HandleConversation(ctx context.Context, args conversation.ChatToolArgs) (*conversation.ChatToolResult, error)
	})
	require.True(t, ok, "adapter should implement HandleConversation method")

	ctx := context.Background()

	t.Run("InitialGreeting", func(t *testing.T) {
		chatArgs := conversation.ChatToolArgs{
			Message: "Hello, I want to containerize my application",
		}

		result, err := adapter.HandleConversation(ctx, chatArgs)
		require.NoError(t, err)
		require.NotNil(t, result)

		message := result.Message
		stage := result.Stage
		sessionID := result.SessionID

		assert.NotEmpty(t, message)
		assert.Equal(t, "preflight", stage)
		assert.NotEmpty(t, sessionID)
		assert.Contains(t, message, "pre-flight")
	})

	t.Run("ConversationContinuation", func(t *testing.T) {
		// First message to establish session
		chatArgs1 := conversation.ChatToolArgs{
			Message: "I want to containerize my Go application",
		}

		result1, err := adapter.HandleConversation(ctx, chatArgs1)
		require.NoError(t, err)
		require.NotNil(t, result1)

		sessionID := result1.SessionID
		require.NotEmpty(t, sessionID)

		// Continue the conversation with session ID
		chatArgs2 := conversation.ChatToolArgs{
			Message:   "Yes, continue with the pre-flight checks",
			SessionID: sessionID,
		}

		result2, err := adapter.HandleConversation(ctx, chatArgs2)
		require.NoError(t, err)
		require.NotNil(t, result2)

		returnedSessionID := result2.SessionID
		assert.Equal(t, sessionID, returnedSessionID)
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test with empty message parameter
		chatArgs := conversation.ChatToolArgs{
			Message:   "", // Empty message should cause error
			SessionID: "test-session",
		}

		result, err := adapter.HandleConversation(ctx, chatArgs)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "message")
	})
}

// TestConversationState tests conversation state management
func TestConversationState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "conversation-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tmpDir,
		StorePath:         filepath.Join(tmpDir, "sessions.db"),
		MaxSessions:       10,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 100,  // 100MB per session
		TotalDiskLimit:    1024 * 1024 * 1024, // 1GB total
		Logger:            logger,
	})
	require.NoError(t, err)
	defer sessionManager.Stop()

	t.Run("ConversationStateCreation", func(t *testing.T) {
		sessionID := "conv-state-test-123"
		sessionInterface, err := sessionManager.GetOrCreateSession(sessionID)
		require.NoError(t, err)

		session, ok := sessionInterface.(*mcp.SessionState)
		require.True(t, ok, "session should be of correct type")

		// Create conversation state
		state := &runtimeconv.ConversationState{
			SessionState: session,
			CurrentStage: mcp.ConversationStagePreFlight, // types.StageInit maps to PreFlight
			History:      []runtimeconv.ConversationTurn{},
			Preferences:  types.UserPreferences{},
			Context:      make(map[string]interface{}),
			Artifacts:    make(map[string]runtimeconv.Artifact),
		}

		assert.Equal(t, mcp.ConversationStagePreFlight, state.CurrentStage)
		assert.Empty(t, state.History)
		assert.NotNil(t, state.Context)
		assert.NotNil(t, state.Artifacts)
	})

	t.Run("ConversationTurnHistory", func(t *testing.T) {
		sessionID := "conv-history-test-456"
		sessionInterface, err := sessionManager.GetOrCreateSession(sessionID)
		require.NoError(t, err)

		session, ok := sessionInterface.(*mcp.SessionState)
		require.True(t, ok, "session should be of correct type")

		state := &runtimeconv.ConversationState{
			SessionState: session,
			CurrentStage: mcp.ConversationStagePreFlight,
			History:      []runtimeconv.ConversationTurn{},
		}

		// Add conversation turns
		turn1 := runtimeconv.ConversationTurn{
			UserInput: "Hello",
			Assistant: "Hi! I'll help you containerize your application.",
			Stage:     mcp.ConversationStagePreFlight,
			Timestamp: time.Now(),
		}

		turn2 := runtimeconv.ConversationTurn{
			UserInput: "Analyze my Go application",
			Assistant: "I'll analyze your Go application. Please provide the repository URL.",
			Stage:     mcp.ConversationStageAnalyze,
			Timestamp: time.Now(),
		}

		state.History = append(state.History, turn1, turn2)

		assert.Len(t, state.History, 2)
		assert.Equal(t, "Hello", state.History[0].UserInput)
		assert.Equal(t, mcp.ConversationStageAnalyze, state.History[1].Stage)
	})
}

// TestConversationStages tests individual conversation stages
func TestConversationStages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "conversation-stages-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tmpDir,
		StorePath:         filepath.Join(tmpDir, "sessions.db"),
		MaxSessions:       10,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 100,  // 100MB per session
		TotalDiskLimit:    1024 * 1024 * 1024, // 1GB total
		Logger:            logger,
	})
	require.NoError(t, err)
	defer sessionManager.Stop()

	preferenceStore, err := utils.NewPreferenceStore(
		filepath.Join(tmpDir, "preferences.db"),
		logger,
		"", // empty encryption passphrase for test
	)
	require.NoError(t, err)
	defer preferenceStore.Close()

	// Create mock tool orchestrator for testing
	mockOrchestrator := &MockConversationOrchestrator{}

	// Create prompt manager
	promptManager := runtimeconv.NewPromptManager(runtimeconv.PromptManagerConfig{
		SessionManager:   sessionManager,
		ToolOrchestrator: mockOrchestrator,
		PreferenceStore:  preferenceStore,
		Logger:           logger,
	})

	ctx := context.Background()

	t.Run("StageInit", func(t *testing.T) {
		sessionID := "stage-init-test"
		_, err := sessionManager.GetOrCreateSession(sessionID)
		require.NoError(t, err)

		response, err := promptManager.ProcessPrompt(ctx, sessionID, "I want to containerize my application")
		require.NoError(t, err)

		assert.NotNil(t, response)
		assert.NotEmpty(t, response.Message)
		assert.Equal(t, mcp.ConversationStagePreFlight, response.Stage)
	})

	t.Run("StageTransition", func(t *testing.T) {
		sessionID := "stage-transition-test"
		_, err := sessionManager.GetOrCreateSession(sessionID)
		require.NoError(t, err)

		// Start with init
		response1, err := promptManager.ProcessPrompt(ctx, sessionID, "Help me containerize my app")
		require.NoError(t, err)
		assert.Equal(t, mcp.ConversationStagePreFlight, response1.Stage)

		// Continue to next stage
		response2, err := promptManager.ProcessPrompt(ctx, sessionID, "Yes, run pre-flight checks")
		require.NoError(t, err)

		// Should progress from preflight
		assert.NotEqual(t, mcp.ConversationStagePreFlight, response2.Stage)
	})
}

// TestConversationOptions tests conversation response options
func TestConversationOptions(t *testing.T) {
	t.Run("OptionCreation", func(t *testing.T) {
		option := runtimeconv.Option{
			ID:          "continue",
			Label:       "Continue with analysis",
			Description: "Proceed to analyze the repository",
			Recommended: true,
		}

		assert.Equal(t, "continue", option.ID)
		assert.Equal(t, "Continue with analysis", option.Label)
		assert.True(t, option.Recommended)
	})

	t.Run("ConversationResponseWithOptions", func(t *testing.T) {
		response := &runtimeconv.ConversationResponse{
			Message: "What would you like to do next?",
			Stage:   mcp.ConversationStageAnalyze,
			Status:  runtimeconv.ResponseStatusSuccess,
			Options: []runtimeconv.Option{
				{
					ID:          "analyze",
					Label:       "Analyze repository",
					Recommended: true,
				},
				{
					ID:          "skip",
					Label:       "Skip analysis",
					Recommended: false,
				},
			},
		}

		assert.Len(t, response.Options, 2)
		assert.True(t, response.Options[0].Recommended)
		assert.False(t, response.Options[1].Recommended)
	})
}

// MockConversationOrchestrator implements conversation.ToolOrchestrator for testing
type MockConversationOrchestrator struct{}

func (m *MockConversationOrchestrator) ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error) {
	return map[string]interface{}{
		"tool":     toolName,
		"success":  true,
		"mock":     true,
		"executed": true,
	}, nil
}

func (m *MockConversationOrchestrator) RegisterTool(name string, tool mcp.Tool) error {
	return nil
}

func (m *MockConversationOrchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	return nil
}

func (m *MockConversationOrchestrator) GetToolMetadata(toolName string) (*mcp.ToolMetadata, error) {
	return &mcp.ToolMetadata{
		Name:        toolName,
		Description: "Mock tool for testing",
		Version:     "1.0.0",
		Category:    "test",
	}, nil
}
