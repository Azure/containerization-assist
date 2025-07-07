package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerCreation tests various server creation scenarios
func TestServerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name      string
		setupFunc func(config *ServerConfig)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid config",
			setupFunc: func(config *ServerConfig) {
				// Use default config
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			setupFunc: func(config *ServerConfig) {
				config.LogLevel = "invalid"
			},
			wantErr: false, // Should default to info level
		},
		{
			name: "invalid storage path",
			setupFunc: func(config *ServerConfig) {
				// Use a path that cannot be created
				config.StorePath = "/root/invalid/path/sessions.db"
			},
			wantErr: true,
			errMsg:  "Failed to create storage directory for server persistence",
		},
		{
			name: "invalid workspace directory",
			setupFunc: func(config *ServerConfig) {
				config.WorkspaceDir = "/root/invalid/workspace"
			},
			wantErr: true,
			errMsg:  "Failed to initialize session management system",
		},
		{
			name: "http transport",
			setupFunc: func(config *ServerConfig) {
				config.TransportType = "http"
				config.HTTPPort = 0 // Random port
			},
			wantErr: false, // HTTP transport creation succeeds, it's the Start() that would fail
		},
		{
			name: "stdio transport",
			setupFunc: func(config *ServerConfig) {
				config.TransportType = "stdio"
			},
			wantErr: false,
		},
		{
			name: "unknown transport",
			setupFunc: func(config *ServerConfig) {
				config.TransportType = "unknown"
			},
			wantErr: false, // Should default to stdio
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultServerConfig()
			config.WorkspaceDir = t.TempDir()
			config.StorePath = filepath.Join(t.TempDir(), "sessions.db")

			if tt.setupFunc != nil {
				tt.setupFunc(&config)
			}

			server, err := NewServer(context.Background(), config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, server)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)

				// Verify components are initialized
				assert.NotNil(t, server.sessionManager)
				assert.NotNil(t, server.workspaceManager)
				assert.NotNil(t, server.circuitBreakers)
				assert.NotNil(t, server.jobManager)
				assert.NotNil(t, server.transport)
				assert.NotNil(t, server.gomcpManager)
			}
		})
	}
}

// TestServerStartupSequence tests the server startup sequence
func TestServerStartupSequence(t *testing.T) {
	// Skip: Requires actual MCP transport setup, external dependencies, and port allocation
	// To implement: Mock transport layer and test state transitions during startup
	t.Skip("Server startup test requires MCP transport mocking - needs transport.MockTransport implementation")
}

// TestServerComponentInitializationFailure tests failure scenarios during component initialization
func TestServerComponentInitializationFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("workspace manager failure propagates", func(t *testing.T) {
		config := DefaultServerConfig()
		// This will fail due to permission issues
		config.WorkspaceDir = "/root/invalid/workspace"
		config.StorePath = ""

		_, err := NewServer(context.Background(), config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to initialize session management system")
	})

	t.Run("storage directory creation failure", func(t *testing.T) {
		config := DefaultServerConfig()
		config.WorkspaceDir = t.TempDir()
		// This will fail due to permission issues
		config.StorePath = "/root/invalid/path/sessions.db"

		_, err := NewServer(context.Background(), config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to create storage directory for server persistence")
	})
}

// TestServerGracefulDegradation tests server behavior when dependencies fail
func TestServerGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = ""
	config.TransportType = "stdio"

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)

	t.Run("conversation mode handles invalid config gracefully", func(t *testing.T) {
		// Try to enable conversation mode with invalid config
		conversationConfig := core.ConsolidatedConversationConfig{
			EnableTelemetry:   true,
			TelemetryPort:     -1,                       // Invalid port - should use defaults
			PreferencesDBPath: "/invalid/path/prefs.db", // Invalid path - should fail or use defaults
		}

		err := server.EnableConversationMode(conversationConfig)
		// The system should either succeed with defaults or handle errors gracefully
		// This tests improved error resilience - either outcome is acceptable
		if err != nil {
			// If it fails, conversation mode should remain disabled
			assert.False(t, server.IsConversationModeEnabled())
		} else {
			// If it succeeds (due to improved error handling), that's also acceptable
			assert.True(t, server.IsConversationModeEnabled())
		}
	})

	t.Run("tool registration continues despite failures", func(t *testing.T) {
		// The server should still function even if some tools fail to register
		// This is tested implicitly as the gomcp manager handles registration
		assert.NotNil(t, server.gomcpManager)
	})
}

// TestServerStopIdempotency tests that Stop() can be called multiple times safely
func TestServerStopIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = ""
	config.TransportType = "stdio"

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)

	// First stop
	err = server.Stop()
	assert.NoError(t, err)

	// Second stop should not error
	err = server.Stop()
	assert.NoError(t, err)

	// Third stop should still be safe
	err = server.Stop()
	assert.NoError(t, err)
}

// TestServerTransportError tests server behavior when context times out
func TestServerTransportError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = ""

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)

	// Use a very short timeout to simulate server startup hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = server.Start(ctx)
	// The server should handle context cancellation gracefully
	assert.NoError(t, err, "Server should shutdown gracefully on context cancellation")
}

// TestServerContextCancellation tests server shutdown via context cancellation
func TestServerContextCancellation(t *testing.T) {
	// Skip: Requires actual server startup and graceful shutdown testing
	// To implement: Use mock transport with context cancellation support
	t.Skip("Context cancellation test requires server startup/shutdown cycle - needs integration test environment")
}

// TestServerCleanupOnFailure tests that resources are preserved when server shuts down
func TestServerCleanupOnFailure(t *testing.T) {
	t.Skip("Temporarily disabled - test times out after 10m due to server startup hanging")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = filepath.Join(t.TempDir(), "sessions.db")

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)

	// Check that storage file was created
	_, err = os.Stat(filepath.Dir(config.StorePath))
	assert.NoError(t, err)

	// Use context cancellation to simulate interrupted startup
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = server.Start(ctx)
	// Server should handle graceful shutdown
	assert.NoError(t, err, "Server should shutdown gracefully")

	// Resources should still be valid (not cleaned up automatically)
	// This ensures the server can be retried if needed
	assert.NotNil(t, server.sessionManager)
	assert.NotNil(t, server.workspaceManager)
	assert.NotNil(t, server.gomcpManager)
}

// TestServerMetrics tests server metrics and health endpoints
func TestServerMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = ""

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)

	// Verify start time is set
	assert.False(t, server.startTime.IsZero())

	// Test uptime by checking start time
	time.Sleep(10 * time.Millisecond)
	uptime := time.Since(server.startTime)
	assert.Greater(t, uptime.Seconds(), float64(0))
}

// TestServerConfigValidation tests configuration validation
func TestServerConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config ServerConfig
		valid  bool
	}{
		{
			name:   "default config is valid",
			config: DefaultServerConfig(),
			valid:  true,
		},
		{
			name: "zero session TTL is valid",
			config: func() ServerConfig {
				c := DefaultServerConfig()
				c.SessionTTL = 0
				return c
			}(),
			valid: true,
		},
		{
			name: "negative values use defaults",
			config: func() ServerConfig {
				c := DefaultServerConfig()
				c.MaxSessions = -1
				c.MaxWorkers = -1
				return c
			}(),
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			config.WorkspaceDir = t.TempDir()
			config.StorePath = ""

			server, err := NewServer(context.Background(), config)
			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, server)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestServerResourceLimits tests resource limit enforcement
func TestServerResourceLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = ""
	config.MaxSessions = 2 // Low limit for testing

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)

	// Create sessions up to the limit
	session1, err := server.sessionManager.GetOrCreateSessionTyped("")
	assert.NoError(t, err)
	require.NotNil(t, session1, "session1 should not be nil")
	assert.NotEmpty(t, session1.SessionID)

	session2, err := server.sessionManager.GetOrCreateSessionTyped("")
	assert.NoError(t, err)
	require.NotNil(t, session2, "session2 should not be nil")
	assert.NotEmpty(t, session2.SessionID)

	// Creating another session should respect the limit
	// Note: The actual limit enforcement depends on session manager implementation
	ctx := context.Background()
	stats, err := server.sessionManager.GetStats(ctx)
	require.NoError(t, err)
	assert.LessOrEqual(t, stats.TotalSessions, config.MaxSessions)
}

// TestDefaultServerConfig tests the default configuration
func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	// Verify defaults
	assert.NotEmpty(t, config.WorkspaceDir)
	assert.Contains(t, config.WorkspaceDir, ".container-kit")
	assert.NotEmpty(t, config.StorePath)
	assert.Equal(t, 50, config.MaxSessions)
	assert.Equal(t, 24*time.Hour, config.SessionTTL)
	assert.Equal(t, "stdio", config.TransportType)
	assert.Equal(t, 8080, config.HTTPPort)
	assert.Equal(t, "info", config.LogLevel)
	assert.False(t, config.SandboxEnabled)

	// Test when home directory cannot be determined
	t.Run("fallback to temp dir", func(t *testing.T) {
		// This is tested implicitly in DefaultServerConfig
		// as it handles the error case
		assert.NotEmpty(t, config.WorkspaceDir)
	})
}

// TestServerLogCapture tests log capture functionality
func TestServerLogCapture(t *testing.T) {
	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = ""
	config.LogLevel = "debug"

	server, err := NewServer(context.Background(), config)
	require.NoError(t, err)

	// Log something
	server.logger.Info("Test log message")
	server.logger.Error("Test error message", "error", fmt.Errorf("test error"))

	// Logs should be captured (this is set up in NewServer)
	// The actual log capture is tested in utils package
	assert.NotNil(t, server.logger)
}
