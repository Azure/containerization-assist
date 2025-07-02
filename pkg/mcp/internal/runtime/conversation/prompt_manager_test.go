package conversation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPromptManager(t *testing.T) {
	logger := zerolog.Nop()

	// Create preference store
	tempDir := t.TempDir()
	prefStore, err := utils.NewPreferenceStore(filepath.Join(tempDir, "prefs.db"), logger, "")
	require.NoError(t, err)
	defer prefStore.Close()

	// Create session manager
	sessionMgr, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tempDir,
		MaxSessions:       10,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,
		TotalDiskLimit:    10 * 1024 * 1024 * 1024,
		StorePath:         filepath.Join(tempDir, "sessions.db"),
		Logger:            logger,
	})
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Create mock tool orchestrator
	toolOrchestrator := &MockToolExecutor{}

	config := PromptManagerConfig{
		SessionManager:   sessionMgr,
		ToolOrchestrator: toolOrchestrator,
		PreferenceStore:  prefStore,
		Logger:           logger,
	}

	pm := NewPromptManager(config)

	assert.NotNil(t, pm)
	assert.NotNil(t, pm.sessionManager)
	assert.NotNil(t, pm.toolOrchestrator)
	assert.NotNil(t, pm.preferenceStore)
	// errorHandler field removed as part of GOMCP-7
	assert.NotNil(t, pm.preFlightChecker)
	assert.NotNil(t, pm.retryManager)
}

func TestPromptManagerProcessPrompt(t *testing.T) {
	logger := zerolog.Nop()

	// Create preference store
	tempDir := t.TempDir()
	prefStore, err := utils.NewPreferenceStore(filepath.Join(tempDir, "prefs.db"), logger, "")
	require.NoError(t, err)
	defer prefStore.Close()

	// Create session manager
	sessionMgr, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tempDir,
		MaxSessions:       10,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,
		TotalDiskLimit:    10 * 1024 * 1024 * 1024,
		StorePath:         filepath.Join(tempDir, "sessions.db"),
		Logger:            logger,
	})
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Create mock tool orchestrator
	toolOrchestrator := &MockToolExecutor{}

	config := PromptManagerConfig{
		SessionManager:   sessionMgr,
		ToolOrchestrator: toolOrchestrator,
		PreferenceStore:  prefStore,
		Logger:           logger,
	}

	pm := NewPromptManager(config)

	// Test processing a simple prompt
	ctx := context.Background()
	sessionID := "test-session-123"
	message := "Hello, I want to containerize my application"

	response, err := pm.ProcessPrompt(ctx, sessionID, message)

	// The exact behavior depends on the implementation, but we should get a response
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, sessionID, response.SessionID)
	assert.NotEmpty(t, response.Message)
}

// MockToolExecutor implements core.ToolOrchestrationExecutor interface for testing
type MockToolExecutor struct{}

func (m *MockToolExecutor) ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error) {
	return map[string]interface{}{
		"tool":     toolName,
		"success":  true,
		"mock":     true,
		"executed": true,
	}, nil
}

func (m *MockToolExecutor) RegisterTool(name string, tool core.Tool) error {
	return nil
}

func (m *MockToolExecutor) ValidateToolArgs(toolName string, args interface{}) error {
	return nil
}

func (m *MockToolExecutor) GetToolMetadata(toolName string) (*core.ToolMetadata, error) {
	return &core.ToolMetadata{
		Name:        toolName,
		Description: "Mock tool for testing",
		Version:     "1.0.0",
		Category:    "test",
	}, nil
}

func TestPromptManagerErrorHandling(t *testing.T) {
	logger := zerolog.Nop()

	// Create preference store
	tempDir := t.TempDir()
	prefStore, err := utils.NewPreferenceStore(filepath.Join(tempDir, "prefs.db"), logger, "")
	require.NoError(t, err)
	defer prefStore.Close()

	// Create session manager
	sessionMgr, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tempDir,
		MaxSessions:       10,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,
		TotalDiskLimit:    10 * 1024 * 1024 * 1024,
		StorePath:         filepath.Join(tempDir, "sessions.db"),
		Logger:            logger,
	})
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Create mock tool orchestrator
	toolOrchestrator := &MockToolExecutor{}

	config := PromptManagerConfig{
		SessionManager:   sessionMgr,
		ToolOrchestrator: toolOrchestrator,
		PreferenceStore:  prefStore,
		Logger:           logger,
	}

	pm := NewPromptManager(config)

	// Test with empty message
	ctx := context.Background()
	response, err := pm.ProcessPrompt(ctx, "test-session", "")

	// Should handle empty message gracefully
	if err != nil {
		// Error is acceptable for empty message
		assert.Error(t, err)
	} else {
		// Or return a response asking for input
		assert.NotNil(t, response)
		assert.NotEmpty(t, response.Message)
	}
}
