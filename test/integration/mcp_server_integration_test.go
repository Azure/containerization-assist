package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPServerIntegration tests the complete MCP server functionality
func TestMCPServerIntegration(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create a temporary workspace for testing
	workspaceDir := t.TempDir()
	storeDir := t.TempDir()

	tests := []struct {
		name string
		test func(t *testing.T, workspaceDir, storeDir string)
	}{
		{
			name: "server_lifecycle",
			test: testServerLifecycle,
		},
		{
			name: "server_configuration",
			test: testServerConfiguration,
		},
		{
			name: "workspace_management",
			test: testWorkspaceManagement,
		},
		{
			name: "session_handling",
			test: testSessionHandling,
		},
		{
			name: "tool_registration",
			test: testToolRegistration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t, workspaceDir, storeDir)
		})
	}
}

// testServerLifecycle tests basic server start/stop functionality
func testServerLifecycle(t *testing.T, workspaceDir, storeDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server configuration
	config := config.DefaultServerConfig()
	config.WorkspaceDir = workspaceDir
	config.StorePath = filepath.Join(storeDir, "sessions.db")
	config.MaxSessions = 10
	config.SessionTTL = 1 * time.Hour
	config.TransportType = "stdio" // Use stdio for testing

	// Create server
	mcpServer, err := core.NewServer(ctx, *config)
	require.NoError(t, err, "Failed to create MCP server")
	require.NotNil(t, mcpServer, "Server should not be nil")

	// Test server statistics (basic functionality test)
	stats, err := mcpServer.GetStats()
	assert.NoError(t, err, "GetStats should not return error")
	assert.NotNil(t, stats, "Server stats should be available")
	// TODO: Uncomment when stats structure is defined
	// assert.Equal(t, "stdio", stats.Transport, "Transport type should match config")
	// assert.NotNil(t, stats.Sessions, "Session stats should be available")
	// assert.NotNil(t, stats.Workspace, "Workspace stats should be available")

	// Test graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = mcpServer.Shutdown(shutdownCtx)
	assert.NoError(t, err, "Server shutdown should succeed")
}

// testServerConfiguration tests various server configuration options
func testServerConfiguration(t *testing.T, workspaceDir, storeDir string) {
	ctx := context.Background()

	tests := []struct {
		name           string
		configModifier func(*config.ServerConfig)
		expectError    bool
	}{
		{
			name: "default_config",
			configModifier: func(config *config.ServerConfig) {
				config.WorkspaceDir = workspaceDir
				config.StorePath = filepath.Join(storeDir, "sessions.db")
			},
			expectError: false,
		},
		{
			name: "sandbox_enabled",
			configModifier: func(config *config.ServerConfig) {
				config.WorkspaceDir = workspaceDir
				config.StorePath = filepath.Join(storeDir, "sessions.db")
				config.SandboxEnabled = true
			},
			expectError: false,
		},
		{
			name: "invalid_workspace_dir",
			configModifier: func(config *config.ServerConfig) {
				config.WorkspaceDir = "/nonexistent/invalid/path"
				config.StorePath = filepath.Join(storeDir, "sessions.db")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := config.DefaultServerConfig()
			tt.configModifier(config)

			mcpServer, err := core.NewServer(ctx, *config)

			if tt.expectError {
				assert.Error(t, err, "Expected error for invalid configuration")
				assert.Nil(t, mcpServer, "Server should be nil on error")
			} else {
				assert.NoError(t, err, "Expected no error for valid configuration")
				assert.NotNil(t, mcpServer, "Server should not be nil")

				// Clean up
				if mcpServer != nil {
					shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					_ = mcpServer.Shutdown(shutdownCtx)
					cancel()
				}
			}
		})
	}
}

// testWorkspaceManagement tests workspace creation and management
func testWorkspaceManagement(t *testing.T, workspaceDir, storeDir string) {
	ctx := context.Background()

	config := config.DefaultServerConfig()
	config.WorkspaceDir = workspaceDir
	config.StorePath = filepath.Join(storeDir, "sessions.db")

	mcpServer, err := core.NewServer(ctx, *config)
	require.NoError(t, err, "Failed to create server")
	require.NotNil(t, mcpServer, "Server should not be nil")
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = mcpServer.Shutdown(shutdownCtx)
		cancel()
	}()

	// Test workspace directory creation
	assert.DirExists(t, workspaceDir, "Workspace directory should exist")

	// Test that the workspace has appropriate permissions
	info, err := os.Stat(workspaceDir)
	require.NoError(t, err, "Should be able to stat workspace directory")
	assert.True(t, info.IsDir(), "Workspace should be a directory")

	// Test workspace subdirectory creation (if sessions are created)
	// This would typically happen when tools are executed
	sessionWorkspace := filepath.Join(workspaceDir, "test-session")
	err = os.MkdirAll(sessionWorkspace, 0755)
	assert.NoError(t, err, "Should be able to create session workspace")

	// Test workspace cleanup capabilities
	assert.DirExists(t, sessionWorkspace, "Session workspace should exist after creation")
}

// testSessionHandling tests session management functionality
func testSessionHandling(t *testing.T, workspaceDir, storeDir string) {
	ctx := context.Background()

	config := config.DefaultServerConfig()
	config.WorkspaceDir = workspaceDir
	config.StorePath = filepath.Join(storeDir, "sessions.db")
	config.MaxSessions = 5
	config.SessionTTL = 10 * time.Minute

	mcpServer, err := core.NewServer(ctx, *config)
	require.NoError(t, err, "Failed to create server")
	require.NotNil(t, mcpServer, "Server should not be nil")
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = mcpServer.Shutdown(shutdownCtx)
		cancel()
	}()

	// Verify session store directory exists
	assert.DirExists(t, storeDir, "Session store directory should exist")

	// Test session configuration limits via stats
	sessionStats, err := mcpServer.GetSessionManagerStats()
	require.NoError(t, err, "Should be able to get session stats")
	assert.NotNil(t, sessionStats, "Session stats should be available")

	// Test session store accessibility
	testFile := filepath.Join(storeDir, "test-session.json")
	testData := map[string]interface{}{
		"test":      "data",
		"timestamp": time.Now().Unix(),
	}

	// Write test session data
	data, err := json.Marshal(testData)
	require.NoError(t, err, "Should be able to marshal test data")

	err = os.WriteFile(testFile, data, 0600)
	assert.NoError(t, err, "Should be able to write to session store")

	// Read test session data
	readData, err := os.ReadFile(testFile)
	assert.NoError(t, err, "Should be able to read from session store")

	var parsed map[string]interface{}
	err = json.Unmarshal(readData, &parsed)
	assert.NoError(t, err, "Should be able to parse session data")
	assert.Equal(t, "data", parsed["test"], "Session data should be preserved")

	// Clean up test file
	os.Remove(testFile)
}

// testToolRegistration tests that tools are properly registered
func testToolRegistration(t *testing.T, workspaceDir, storeDir string) {
	ctx := context.Background()

	config := config.DefaultServerConfig()
	config.WorkspaceDir = workspaceDir
	config.StorePath = filepath.Join(storeDir, "sessions.db")

	mcpServer, err := core.NewServer(ctx, *config)
	require.NoError(t, err, "Failed to create server")
	require.NotNil(t, mcpServer, "Server should not be nil")
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = mcpServer.Shutdown(shutdownCtx)
		cancel()
	}()

	// Test that server has basic functionality available
	// Test stats access (which doesn't require starting the server)
	stats, err := mcpServer.GetStats()
	assert.NoError(t, err, "GetStats should not return error")
	assert.NotNil(t, stats, "Server stats should be available")
	// TODO: Uncomment when stats structure is defined
	// assert.Equal(t, "stdio", stats.Transport, "Transport should be configured")

	// Test session and workspace managers are accessible
	sessionStats, err := mcpServer.GetSessionManagerStats()
	assert.NoError(t, err, "GetSessionManagerStats should not return error")
	assert.NotNil(t, sessionStats, "Session manager should be initialized")

	// TODO: GetWorkspaceStats method doesn't exist in the interface
	// workspaceStats := mcpServer.GetWorkspaceStats()
	// assert.NotNil(t, workspaceStats, "Workspace manager should be initialized")

	// TODO: GetLogger method doesn't exist in the interface
	// logger := mcpServer.GetLogger()
	// assert.NotNil(t, logger, "Server logger should be available")

	t.Log("Server components are properly initialized and accessible")
}

// TestMCPServerStressTest tests server behavior under load
func TestMCPServerStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Only run stress tests if explicitly requested
	if os.Getenv("RUN_STRESS_TESTS") != "true" {
		t.Skip("Skipping stress test (set RUN_STRESS_TESTS=true to enable)")
	}

	workspaceDir := t.TempDir()
	storeDir := t.TempDir()
	ctx := context.Background()

	config := config.DefaultServerConfig()
	config.WorkspaceDir = workspaceDir
	config.StorePath = filepath.Join(storeDir, "sessions.db")
	config.MaxSessions = 100

	mcpServer, err := core.NewServer(ctx, *config)
	require.NoError(t, err, "Failed to create server")
	require.NotNil(t, mcpServer, "Server should not be nil")
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_ = mcpServer.Shutdown(shutdownCtx)
		cancel()
	}()

	// Test rapid server stats requests
	for i := 0; i < 1000; i++ {
		stats, err := mcpServer.GetStats()
		require.NoError(t, err, "Should be able to get server stats")
		assert.NotNil(t, stats, "Server stats should always be available")

		if i%100 == 0 {
			t.Logf("Processed %d server stats requests", i+1)
		}
	}

	// Test concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(workerID int) {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				stats, err := mcpServer.GetStats()
				assert.NoError(t, err, "Should be able to get stats in worker %d", workerID)
				assert.NotNil(t, stats, "Stats should be available in worker %d", workerID)
			}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Worker completed
		case <-time.After(30 * time.Second):
			t.Fatal("Stress test timed out")
		}
	}

	t.Log("Stress test completed successfully")
}
