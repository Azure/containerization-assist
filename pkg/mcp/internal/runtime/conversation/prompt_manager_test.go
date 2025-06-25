package conversation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/session/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/preference"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPromptManager(t *testing.T) {
	logger := zerolog.Nop()

	// Create preference store
	tempDir := t.TempDir()
	prefStore, err := preference.NewPreferenceStore(filepath.Join(tempDir, "prefs.db"), logger, "")
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
	toolOrchestrator := &MockToolOrchestrator{}

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
	prefStore, err := preference.NewPreferenceStore(filepath.Join(tempDir, "prefs.db"), logger, "")
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
	toolOrchestrator := &MockToolOrchestrator{}

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

// MockToolOrchestrator implements ToolOrchestrator interface for testing
type MockToolOrchestrator struct{}

func (m *MockToolOrchestrator) ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error) {
	return map[string]interface{}{
		"tool":     toolName,
		"success":  true,
		"mock":     true,
		"executed": true,
	}, nil
}

func (m *MockToolOrchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	return nil
}

func (m *MockToolOrchestrator) GetToolMetadata(toolName string) (*orchestration.ToolMetadata, error) {
	return &orchestration.ToolMetadata{
		Name:        toolName,
		Description: "Mock tool for testing",
	}, nil
}

func TestPromptManagerErrorHandling(t *testing.T) {
	logger := zerolog.Nop()

	// Create preference store
	tempDir := t.TempDir()
	prefStore, err := preference.NewPreferenceStore(filepath.Join(tempDir, "prefs.db"), logger, "")
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
	toolOrchestrator := &MockToolOrchestrator{}

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
