package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/registrar"
	"github.com/Azure/container-kit/pkg/mcp/application/transport"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	// "github.com/Azure/container-kit/pkg/wire" // Temporarily disabled to avoid import cycles
	"github.com/mark3labs/mcp-go/server"
)

// serverImpl represents the consolidated MCP server implementation
type serverImpl struct {
	deps      *Dependencies
	startTime time.Time

	mcpServer        *server.MCPServer
	isMcpInitialized bool

	shutdownMutex  sync.Mutex
	isShuttingDown bool
}

// ConversationComponents represents conversation mode components
type ConversationComponents struct {
	_ bool // placeholder field
}

// registerComponents registers all tools, prompts, and resources
func (s *serverImpl) registerComponents() error {
	if s.mcpServer == nil {
		return errors.New(errors.CodeInternalError, "server", "mcp server not initialized", nil)
	}

	// Create and use unified registrar
	registrar := registrar.NewRegistrar(s.deps.Logger, s.deps.ResourceStore)
	if err := registrar.RegisterAll(s.mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "server", "failed to register components", err)
	}

	s.deps.Logger.Info("All components registered successfully")
	return nil
}

// NewMCPServer creates a new MCP server with the given options using Wire dependency injection.
// This is the primary public API for creating MCP servers
func NewMCPServer(ctx context.Context, logger *slog.Logger, opts ...Option) (api.MCPServer, error) {
	// Extract configuration from options (if provided)
	tempDeps := &Dependencies{Config: workflow.DefaultServerConfig()}
	for _, opt := range opts {
		opt(tempDeps)
	}
	config := tempDeps.Config

	// Ensure directories exist
	if config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(config.StorePath), 0o755); err != nil {
			logger.Error("Failed to create storage directory", "error", err, "path", config.StorePath)
			return nil, errors.New(errors.CodeIoError, "server", fmt.Sprintf("failed to create storage directory %s", config.StorePath), err)
		}
	}

	if config.WorkspaceDir != "" {
		if err := os.MkdirAll(config.WorkspaceDir, 0o755); err != nil {
			logger.Error("Failed to create workspace directory", "error", err, "path", config.WorkspaceDir)
			return nil, errors.New(errors.CodeIoError, "server", fmt.Sprintf("failed to create workspace directory %s", config.WorkspaceDir), err)
		}
	}

	// Use Wire for dependency injection
	// Check if custom config was provided
	var server api.MCPServer
	var err error

	if tempDeps.Config.WorkspaceDir != workflow.DefaultServerConfig().WorkspaceDir ||
		tempDeps.Config.StorePath != workflow.DefaultServerConfig().StorePath ||
		tempDeps.Config.MaxSessions != workflow.DefaultServerConfig().MaxSessions {
		// Custom config provided, use InitializeServerWithConfig
		server, err = initializeServerWithCustomConfig(logger, config)
	} else {
		// Use default config from environment
		server, err = initializeServerFromEnv(logger)
	}

	if err != nil {
		logger.Error("Failed to initialize server with Wire", "error", err)
		return nil, fmt.Errorf("failed to initialize server: %w", err)
	}

	logger.Info("MCP Server initialized successfully with Wire",
		"transport", config.TransportType,
		"workspace_dir", config.WorkspaceDir,
		"max_sessions", config.MaxSessions)

	return server, nil
}

// initializeServerFromEnv initializes server using environment-based configuration
func initializeServerFromEnv(logger *slog.Logger) (api.MCPServer, error) {
	// Import the wire package to access the generated injector
	return initializeServer(logger)
}

// initializeServerWithCustomConfig initializes server with custom configuration
func initializeServerWithCustomConfig(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	// Import the wire package to access the generated injector
	return initializeServerWithConfig(logger, config)
}

// initializeServer wraps the Wire-generated injector with conversion to application.Dependencies
func initializeServer(logger *slog.Logger) (api.MCPServer, error) {
	// Phase 1: Wire infrastructure is ready but temporarily disabled due to import cycle
	// Will be enabled in Phase 1b after resolving architectural pattern
	return nil, fmt.Errorf("Wire injection temporarily disabled - see Phase 1b of implementation plan")
}

// initializeServerWithConfig wraps the Wire-generated injector with custom config
func initializeServerWithConfig(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	// Phase 1: Wire infrastructure is ready but temporarily disabled due to import cycle
	// Will be enabled in Phase 1b after resolving architectural pattern
	return nil, fmt.Errorf("Wire injection temporarily disabled - see Phase 1b of implementation plan")
}

// Start starts the MCP server
func (s *serverImpl) Start(ctx context.Context) error {
	s.deps.Logger.Info("Starting Container Kit MCP Server",
		"transport", s.deps.Config.TransportType,
		"workspace_dir", s.deps.Config.WorkspaceDir,
		"max_sessions", s.deps.Config.MaxSessions)

	if err := s.deps.SessionManager.StartCleanupRoutine(ctx); err != nil {
		s.deps.Logger.Error("Failed to start cleanup routine", "error", err)
		return err
	}

	// Start resource store cleanup routine
	s.deps.ResourceStore.StartCleanupRoutine(30*time.Minute, 24*time.Hour)

	// Initialize mcp-go server directly without manager abstraction
	if !s.isMcpInitialized {
		s.deps.Logger.Info("Initializing mcp-go server")

		// Create mcp-go server with capabilities
		s.mcpServer = server.NewMCPServer(
			"container-kit-mcp",
			"1.0.0",
			server.WithResourceCapabilities(true, true),
			server.WithPromptCapabilities(true),
			server.WithToolCapabilities(true),
			server.WithLogging(),
		)

		if s.mcpServer == nil {
			return errors.New(errors.CodeInternalError, "transport", "failed to create mcp-go server", nil)
		}

		// Register all components
		if err := s.registerComponents(); err != nil {
			return errors.New(errors.CodeToolExecutionFailed, "transport", "failed to register components with mcp-go", err)
		}

		// Register chat modes for Copilot integration
		if err := s.RegisterChatModes(); err != nil {
			s.deps.Logger.Warn("Failed to register chat modes", "error", err)
			// Don't fail server startup for this
		}

		s.isMcpInitialized = true
		s.deps.Logger.Info("MCP-GO server initialized successfully")
	}

	// Use transport manager to start appropriate transport
	transportType := transport.TransportType(s.deps.Config.TransportType)
	transportManager := transport.NewManager(s.deps.Logger, transportType, 0)
	return transportManager.Start(ctx, s.mcpServer)
}

// Shutdown gracefully shuts down the server with proper context handling
func (s *serverImpl) Shutdown(ctx context.Context) error {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()

	if s.isShuttingDown {
		return nil // Already shutting down
	}
	s.isShuttingDown = true

	s.deps.Logger.Info("Gracefully shutting down MCP Server")

	// Stop session manager with context awareness
	done := make(chan error, 1)
	go func() {
		if err := s.deps.SessionManager.Stop(ctx); err != nil {
			s.deps.Logger.Error("Failed to stop session manager", "error", err)
			done <- err
			return
		}
		done <- nil
	}()

	// Wait for shutdown or context cancellation
	select {
	case <-ctx.Done():
		s.deps.Logger.Warn("Shutdown cancelled by context", "error", ctx.Err())
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}

	s.deps.Logger.Info("MCP Server shutdown complete")
	return nil
}

// Stop stops the MCP server (implements api.MCPServer)
func (s *serverImpl) Stop(ctx context.Context) error {
	// Use the provided context for shutdown
	return s.Shutdown(ctx)
}

// EnableConversationMode enables conversation mode (workflow-focused server - no-op)
func (s *serverImpl) EnableConversationMode(_ interface{}) error {
	s.deps.Logger.Info("Conversation mode not supported in workflow-focused server")
	return nil
}

// IsConversationModeEnabled returns whether conversation mode is enabled (always false)
func (s *serverImpl) IsConversationModeEnabled() bool {
	return false // Workflow-focused server doesn't support conversation mode
}

// GetName returns the server name
func (s *serverImpl) GetName() string {
	return "container-kit-mcp-server"
}

// GetStats returns server statistics
func (s *serverImpl) GetStats() (interface{}, error) {
	return map[string]interface{}{
		"name":              s.GetName(),
		"uptime":            time.Since(s.startTime).String(),
		"status":            "running",
		"session_count":     s.getSessionCount(),
		"transport_type":    s.deps.Config.TransportType,
		"conversation_mode": s.IsConversationModeEnabled(),
	}, nil
}

// getSessionCount returns the current number of sessions
func (s *serverImpl) getSessionCount() int {
	if s.deps.SessionManager == nil {
		return 0
	}

	ctx := context.Background()
	sessions, err := s.deps.SessionManager.ListSessionsTyped(ctx)
	if err != nil {
		s.deps.Logger.Warn("Failed to get session count", "error", err)
		return 0
	}

	return len(sessions)
}

// GetSessionManagerStats returns session manager statistics
func (s *serverImpl) GetSessionManagerStats() (interface{}, error) {
	if s.deps.SessionManager != nil {
		ctx := context.Background()
		sessions, err := s.deps.SessionManager.ListSessionsTyped(ctx)
		if err != nil {
			s.deps.Logger.Warn("Failed to get session list for stats", "error", err)
			return map[string]interface{}{
				"error":        "failed to retrieve session stats",
				"max_sessions": s.deps.Config.MaxSessions,
			}, nil
		}

		// Count active sessions (not expired and not failed)
		activeSessions := 0
		for _, session := range sessions {
			if time.Now().Before(session.ExpiresAt) && session.Status != "failed" {
				activeSessions++
			}
		}

		return map[string]interface{}{
			"active_sessions": activeSessions,
			"total_sessions":  len(sessions),
			"max_sessions":    s.deps.Config.MaxSessions,
		}, nil
	}
	return map[string]interface{}{
		"error": "session manager not initialized",
	}, nil
}

// RegisterChatModes registers custom chat modes for Copilot integration
func (s *serverImpl) RegisterChatModes() error {
	s.deps.Logger.Info("Chat mode support enabled via standard MCP protocol",
		"available_tools", GetChatModeFunctions())

	return nil
}
