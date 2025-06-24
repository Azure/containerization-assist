package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/ops"
	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/preference"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/Azure/container-copilot/pkg/mcp/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPServerBasics tests basic MCP server functionality
func TestMCPServerBasics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	tmpDir, err := os.MkdirTemp("", "mcp-server-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create server configuration
	config := ServerConfig{
		WorkspaceDir:      tmpDir,
		MaxSessions:       10,
		SessionTTL:        1 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,      // 1GB
		TotalDiskLimit:    10 * 1024 * 1024 * 1024, // 10GB
		StorePath:         filepath.Join(tmpDir, "sessions.db"),
		TransportType:     "stdio",
		LogLevel:          "error", // Reduce noise in tests
	}

	t.Run("ServerCreation", func(t *testing.T) {
		// Create config with unique storage path for this subtest
		testConfig := config
		testConfig.StorePath = filepath.Join(tmpDir, "sessions_creation.db")

		server, err := NewServer(testConfig)
		require.NoError(t, err)
		assert.NotNil(t, server)

		// Check that basic components are initialized
		assert.False(t, server.IsConversationModeEnabled())

		err = server.Stop()
		assert.NoError(t, err)
	})

	t.Run("ConversationModeEnabled", func(t *testing.T) {
		// Create config with unique storage path for this subtest
		testConfig := config
		testConfig.StorePath = filepath.Join(tmpDir, "sessions_conversation.db")

		server, err := NewServer(testConfig)
		require.NoError(t, err)
		defer server.Stop()

		// Enable conversation mode
		convConfig := ConversationConfig{
			EnableTelemetry:   false, // Disable for tests
			PreferencesDBPath: filepath.Join(tmpDir, "preferences.db"),
		}

		err = server.EnableConversationMode(convConfig)
		require.NoError(t, err)

		assert.True(t, server.IsConversationModeEnabled())
		assert.NotNil(t, server.GetConversationAdapter())
	})

	t.Run("ConversationModeWithTelemetry", func(t *testing.T) {
		// Create config with unique storage path for this subtest
		testConfig := config
		testConfig.StorePath = filepath.Join(tmpDir, "sessions_telemetry.db")

		server, err := NewServer(testConfig)
		require.NoError(t, err)
		defer server.Stop()

		// Enable conversation mode with telemetry
		convConfig := ConversationConfig{
			EnableTelemetry:   true,
			TelemetryPort:     0, // Use random port for testing
			PreferencesDBPath: filepath.Join(tmpDir, "preferences_telemetry.db"),
		}

		err = server.EnableConversationMode(convConfig)
		require.NoError(t, err)

		assert.True(t, server.IsConversationModeEnabled())
		assert.NotNil(t, server.GetTelemetry())
	})
}

// TestSessionManager tests session management functionality
func TestSessionManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "session-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tmpDir,
		MaxSessions:       5,
		SessionTTL:        1 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,
		TotalDiskLimit:    5 * 1024 * 1024 * 1024,
		StorePath:         filepath.Join(tmpDir, "sessions.db"),
		Logger:            logger,
	})
	require.NoError(t, err)
	defer sessionManager.Stop()

	t.Run("CreateAndRetrieveSession", func(t *testing.T) {
		sessionID := "test-session-123"

		// Create session
		session, err := sessionManager.GetOrCreateSession(sessionID)
		require.NoError(t, err)
		assert.Equal(t, sessionID, session.SessionID)
		assert.NotEmpty(t, session.WorkspaceDir)

		// Retrieve session
		retrieved, err := sessionManager.GetSession(sessionID)
		require.NoError(t, err)
		assert.Equal(t, sessionID, retrieved.SessionID)
		assert.Equal(t, session.WorkspaceDir, retrieved.WorkspaceDir)
	})

	t.Run("SessionPersistence", func(t *testing.T) {
		sessionID := "persist-test-456"

		// Create session with some data
		session, err := sessionManager.GetOrCreateSession(sessionID)
		require.NoError(t, err)

		// Update session with test data
		session.RepoURL = "https://github.com/test/repo"
		session.RepoAnalysis = map[string]interface{}{
			"language":  "Go",
			"framework": "gin",
		}

		err = sessionManager.UpdateSession(sessionID, func(s *sessiontypes.SessionState) {
			s.RepoAnalysis = session.RepoAnalysis
		})
		require.NoError(t, err)

		// Retrieve and verify persistence
		retrieved, err := sessionManager.GetSession(sessionID)
		require.NoError(t, err)
		assert.Equal(t, "https://github.com/test/repo", retrieved.RepoURL)
		assert.Equal(t, "Go", retrieved.RepoAnalysis["language"])
		assert.Equal(t, "gin", retrieved.RepoAnalysis["framework"])
	})

	t.Run("SessionExpiration", func(t *testing.T) {
		// Create session manager with very short TTL for testing
		shortTTLManager, err := session.NewSessionManager(session.SessionManagerConfig{
			WorkspaceDir:      tmpDir,
			MaxSessions:       5,
			SessionTTL:        100 * time.Millisecond,
			MaxDiskPerSession: 1024 * 1024 * 1024,
			TotalDiskLimit:    5 * 1024 * 1024 * 1024,
			StorePath:         filepath.Join(tmpDir, "short_sessions.db"),
			Logger:            logger,
		})
		require.NoError(t, err)
		defer shortTTLManager.Stop()

		sessionID := "expire-test-789"

		// Create session
		_, err = shortTTLManager.GetOrCreateSession(sessionID)
		require.NoError(t, err)

		// Verify session exists
		_, err = shortTTLManager.GetSession(sessionID)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		// Run garbage collection to clean expired sessions
		err = shortTTLManager.GarbageCollect()
		require.NoError(t, err)

		// Session should be gone after garbage collection
		_, err = shortTTLManager.GetSession(sessionID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestConversationComponents tests conversation mode components
func TestConversationComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "conversation-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	t.Run("PreferenceStore", func(t *testing.T) {
		prefsPath := filepath.Join(tmpDir, "preferences.db")
		preferenceStore, err := preference.NewPreferenceStore(prefsPath, logger, "")
		require.NoError(t, err)
		defer preferenceStore.Close()

		userID := "test-user-123"

		// Test storing and retrieving preferences
		prefs := &preference.GlobalPreferences{
			UserID:              userID,
			DefaultOptimization: "size",
			DefaultNamespace:    "test-namespace",
			PreferredRegistry:   "docker.io",
		}

		err = preferenceStore.SaveUserPreferences(prefs)
		require.NoError(t, err)

		retrieved, err := preferenceStore.GetUserPreferences(userID)
		require.NoError(t, err)
		assert.Equal(t, "size", retrieved.DefaultOptimization)
		assert.Equal(t, "test-namespace", retrieved.DefaultNamespace)
		assert.Equal(t, "docker.io", retrieved.PreferredRegistry)
	})

	t.Run("PreFlightChecker", func(t *testing.T) {
		checker := ops.NewPreFlightChecker(logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := checker.RunChecks(ctx)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// At minimum, should have some checks
		assert.NotEmpty(t, result.Checks)
	})

	t.Run("ToolOrchestrator", func(t *testing.T) {
		// Create session manager for orchestrator
		sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
			WorkspaceDir:      tmpDir,
			StorePath:         filepath.Join(tmpDir, "orchestrator_sessions.db"),
			MaxSessions:       10,
			SessionTTL:        24 * time.Hour,
			MaxDiskPerSession: 1024 * 1024 * 100,  // 100MB per session
			TotalDiskLimit:    1024 * 1024 * 1024, // 1GB total
			Logger:            logger,
		})
		require.NoError(t, err)
		defer sessionManager.Stop()

		// Remove old orchestrator config as it's no longer needed

		// Create mock tool registry and session manager for testing
		toolRegistry := orchestration.NewMCPToolRegistry(logger)
		sessionMgrImpl := &MockSessionManager{}
		orchestrator := orchestration.NewMCPToolOrchestrator(toolRegistry, sessionMgrImpl, logger)
		assert.NotNil(t, orchestrator)
	})
}

// TestUtilities tests the shared utilities
func TestUtilities(t *testing.T) {
	t.Run("ExtractBaseImage", func(t *testing.T) {
		dockerfile := `FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o main .
CMD ["./main"]`

		baseImage := utils.ExtractBaseImage(dockerfile)
		assert.Equal(t, "golang:1.21-alpine", baseImage)
	})

	t.Run("FormatBytes", func(t *testing.T) {
		tests := []struct {
			bytes    int64
			expected string
		}{
			{512, "512 B"},
			{1024, "1.0 KB"},
			{1536, "1.5 KB"},
			{1048576, "1.0 MB"},
			{1073741824, "1.0 GB"},
		}

		for _, test := range tests {
			result := utils.FormatBytes(test.bytes)
			assert.Equal(t, test.expected, result)
		}
	})

	t.Run("GetStringFromMap", func(t *testing.T) {
		testMap := map[string]interface{}{
			"string_key": "string_value",
			"int_key":    42,
			"bool_key":   true,
		}

		// Test string extraction
		result := utils.GetStringFromMap(testMap, "string_key")
		assert.Equal(t, "string_value", result)

		// Test non-existent key
		result = utils.GetStringFromMap(testMap, "missing_key")
		assert.Equal(t, "", result)

		// Test wrong type
		result = utils.GetStringFromMap(testMap, "int_key")
		assert.Equal(t, "", result)
	})
}

// TestErrorHandling tests error handling utilities
func TestErrorHandling(t *testing.T) {
	t.Run("WrapError", func(t *testing.T) {
		originalErr := assert.AnError
		wrappedErr := utils.WrapError(originalErr, "test operation")

		assert.Error(t, wrappedErr)
		assert.Contains(t, wrappedErr.Error(), "failed to test operation")
		assert.Contains(t, wrappedErr.Error(), originalErr.Error())
	})

	t.Run("NewError", func(t *testing.T) {
		err := utils.NewError("test operation", "something went wrong")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to test operation")
		assert.Contains(t, err.Error(), "something went wrong")
	})

	t.Run("WrapErrorNil", func(t *testing.T) {
		wrappedErr := utils.WrapError(nil, "test operation")
		assert.NoError(t, wrappedErr)
	})
}

// MockSessionManager implements orchestration.SessionManager for testing
type MockSessionManager struct{}

func (m *MockSessionManager) GetSession(sessionID string) (interface{}, error) {
	return &sessiontypes.SessionState{
		SessionID: sessionID,
	}, nil
}

func (m *MockSessionManager) UpdateSession(session interface{}) error {
	return nil
}
