// Package lifecycle provides server lifecycle management functionality
package lifecycle

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/service/bootstrap"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
	"github.com/mark3labs/mcp-go/server"
)

// LifecycleManager handles server startup and shutdown logic
type LifecycleManager struct {
	logger           *slog.Logger
	config           workflow.ServerConfig
	sessionManager   session.OptimizedSessionManager
	bootstrapper     *bootstrap.Bootstrapper
	mcpServer        *server.MCPServer
	isMcpInitialized bool
	shutdownMutex    sync.Mutex
	isShuttingDown   bool
	startTime        time.Time
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(
	logger *slog.Logger,
	config workflow.ServerConfig,
	sessionManager session.OptimizedSessionManager,
	bootstrapper *bootstrap.Bootstrapper,
) *LifecycleManager {
	return &LifecycleManager{
		logger:         logger,
		config:         config,
		sessionManager: sessionManager,
		bootstrapper:   bootstrapper,
		startTime:      time.Now(),
	}
}

// Start starts the MCP server with full initialization
func (m *LifecycleManager) Start(ctx context.Context) error {
	m.logger.Info("Starting Container Kit MCP Server",
		"workspace_dir", m.config.WorkspaceDir,
		"max_sessions", m.config.MaxSessions)

	// Initialize directories first
	if err := m.bootstrapper.InitializeDirectories(); err != nil {
		return err
	}

	// Session manager handles cleanup automatically
	m.logger.Info("Session cleanup handled automatically by OptimizedSessionManager")

	// Initialize mcp-go server if not already done
	if !m.isMcpInitialized {
		if err := m.initializeMCPServer(); err != nil {
			return err
		}
	}

	// Start stdio transport directly
	m.logger.Info("Starting stdio transport")
	return server.ServeStdio(m.mcpServer)
}

// initializeMCPServer handles MCP server initialization and registration
func (m *LifecycleManager) initializeMCPServer() error {
	m.logger.Info("Initializing mcp-go server")

	// Create the MCP server
	m.mcpServer = m.bootstrapper.CreateMCPServer()
	if m.mcpServer == nil {
		return errors.New(errors.CodeInternalError, "lifecycle", "failed to create mcp-go server", nil)
	}

	// Register all components
	if err := m.bootstrapper.RegisterComponents(m.mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "lifecycle", "failed to register components with mcp-go", err)
	}

	// Register chat modes for Copilot integration
	if err := m.bootstrapper.RegisterChatModes(); err != nil {
		m.logger.Warn("Failed to register chat modes", "error", err)
		// Don't fail server startup for this
	}

	m.isMcpInitialized = true
	m.logger.Info("MCP-GO server initialized successfully")
	return nil
}

// Shutdown gracefully shuts down the server with proper context handling
func (m *LifecycleManager) Shutdown(ctx context.Context) error {
	m.shutdownMutex.Lock()
	defer m.shutdownMutex.Unlock()

	if m.isShuttingDown {
		return nil // Already shutting down
	}
	m.isShuttingDown = true

	m.logger.Info("Gracefully shutting down MCP Server")

	// Stop session manager with context awareness
	done := make(chan error, 1)
	go func() {
		if err := m.sessionManager.Stop(ctx); err != nil {
			m.logger.Error("Failed to stop session manager", "error", err)
			done <- err
			return
		}
		done <- nil
	}()

	// Wait for shutdown or context cancellation
	select {
	case <-ctx.Done():
		m.logger.Warn("Shutdown cancelled by context", "error", ctx.Err())
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}

	m.logger.Info("MCP Server shutdown complete")
	return nil
}

// GetUptime returns the server uptime
func (m *LifecycleManager) GetUptime() time.Duration {
	return time.Since(m.startTime)
}

// IsInitialized returns whether the MCP server is initialized
func (m *LifecycleManager) IsInitialized() bool {
	return m.isMcpInitialized
}

// IsShuttingDown returns whether the server is in shutdown process
func (m *LifecycleManager) IsShuttingDown() bool {
	m.shutdownMutex.Lock()
	defer m.shutdownMutex.Unlock()
	return m.isShuttingDown
}
