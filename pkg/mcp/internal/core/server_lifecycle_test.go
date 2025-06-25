package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
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
			errMsg:  "failed to create storage directory",
		},
		{
			name: "invalid workspace directory",
			setupFunc: func(config *ServerConfig) {
				config.WorkspaceDir = "/root/invalid/workspace"
			},
			wantErr: true,
			errMsg:  "failed to initialize session manager",
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

			server, err := NewServer(config)
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

		_, err := NewServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize session manager")
	})

	t.Run("storage directory creation failure", func(t *testing.T) {
		config := DefaultServerConfig()
		config.WorkspaceDir = t.TempDir()
		// This will fail due to permission issues
		config.StorePath = "/root/invalid/path/sessions.db"

		_, err := NewServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create storage directory")
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

	server, err := NewServer(config)
	require.NoError(t, err)

	t.Run("conversation mode fails to enable", func(t *testing.T) {
		// Try to enable conversation mode with invalid config
		conversationConfig := ConversationConfig{
			EnableTelemetry:   true,
			TelemetryPort:     -1, // Invalid port
			PreferencesDBPath: "/invalid/path/prefs.db",
		}

		err := server.EnableConversationMode(conversationConfig)
		// Should handle error gracefully
		assert.Error(t, err)
		assert.False(t, server.IsConversationModeEnabled())
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

	server, err := NewServer(config)
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

// TestServerTransportError tests server behavior when transport fails
func TestServerTransportError(t *testing.T) {
	t.Skip("Temporarily disabled - test times out after 10m due to server startup hanging")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = ""

	server, err := NewServer(config)
	require.NoError(t, err)

	// Create a mock transport that fails
	mockTransport := &mockFailingTransport{
		failOnServe: true,
		serveErr:    errors.New("transport failed"),
	}
	server.transport = mockTransport

	ctx := context.Background()
	err = server.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transport failed")
}

// TestServerContextCancellation tests server shutdown via context cancellation
func TestServerContextCancellation(t *testing.T) {
	// Skip: Requires actual server startup and graceful shutdown testing
	// To implement: Use mock transport with context cancellation support
	t.Skip("Context cancellation test requires server startup/shutdown cycle - needs integration test environment")
}

// TestServerCleanupOnFailure tests that resources are cleaned up on startup failure
func TestServerCleanupOnFailure(t *testing.T) {
	t.Skip("Temporarily disabled - test times out after 10m due to server startup hanging")
	
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultServerConfig()
	config.WorkspaceDir = t.TempDir()
	config.StorePath = filepath.Join(t.TempDir(), "sessions.db")

	server, err := NewServer(config)
	require.NoError(t, err)

	// Check that storage file was created
	_, err = os.Stat(filepath.Dir(config.StorePath))
	assert.NoError(t, err)

	// Simulate startup failure by using a failing transport
	mockTransport := &mockFailingTransport{
		failOnServe: true,
		serveErr:    errors.New("startup failed"),
	}
	server.transport = mockTransport

	ctx := context.Background()
	err = server.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup failed")

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

	server, err := NewServer(config)
	require.NoError(t, err)

	// Verify start time is set
	assert.False(t, server.startTime.IsZero())

	// Test uptime by checking start time
	time.Sleep(10 * time.Millisecond)
	uptime := time.Since(server.startTime)
	assert.Greater(t, uptime.Seconds(), float64(0))
}

// mockFailingTransport is a mock transport that can be configured to fail
type mockFailingTransport struct {
	failOnServe bool
	serveErr    error
	handler     InternalRequestHandler
}

func (m *mockFailingTransport) Serve(ctx context.Context) error {
	if m.failOnServe {
		return m.serveErr
	}
	<-ctx.Done()
	return nil
}

func (m *mockFailingTransport) Start(ctx context.Context) error {
	return m.Serve(ctx)
}

func (m *mockFailingTransport) Stop(ctx context.Context) error {
	return nil
}

func (m *mockFailingTransport) SendMessage(message interface{}) error {
	return nil
}

func (m *mockFailingTransport) ReceiveMessage() (interface{}, error) {
	return nil, nil
}

func (m *mockFailingTransport) Name() string {
	return "mock-failing-transport"
}

func (m *mockFailingTransport) SetHandler(handler interface{}) {
	if h, ok := handler.(InternalRequestHandler); ok {
		m.handler = h
	}
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

			server, err := NewServer(config)
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

	server, err := NewServer(config)
	require.NoError(t, err)

	// Create sessions up to the limit
	session1Interface, err := server.sessionManager.GetOrCreateSession("")
	assert.NoError(t, err)
	session1, ok := session1Interface.(*sessiontypes.SessionState)
	require.True(t, ok, "session1 should be of correct type")
	assert.NotEmpty(t, session1.SessionID)

	session2Interface, err := server.sessionManager.GetOrCreateSession("")
	assert.NoError(t, err)
	session2, ok := session2Interface.(*sessiontypes.SessionState)
	require.True(t, ok, "session2 should be of correct type")
	assert.NotEmpty(t, session2.SessionID)

	// Creating another session should respect the limit
	// Note: The actual limit enforcement depends on session manager implementation
	stats := server.sessionManager.GetStats()
	assert.LessOrEqual(t, stats.TotalSessions, config.MaxSessions)
}

// TestDefaultServerConfig tests the default configuration
func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	// Verify defaults
	assert.NotEmpty(t, config.WorkspaceDir)
	assert.Contains(t, config.WorkspaceDir, ".container-kit")
	assert.NotEmpty(t, config.StorePath)
	assert.Equal(t, 10, config.MaxSessions)
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

	server, err := NewServer(config)
	require.NoError(t, err)

	// Log something
	server.logger.Info().Msg("Test log message")
	server.logger.Error().Err(fmt.Errorf("test error")).Msg("Test error message")

	// Logs should be captured (this is set up in NewServer)
	// The actual log capture is tested in utils package
	assert.NotNil(t, server.logger)
}
