// Package application provides the application service layer for the Container Kit MCP server.
// This layer orchestrates domain services and infrastructure components to implement
// the complete MCP (Model Context Protocol) server functionality.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/bootstrap"
	"github.com/Azure/container-kit/pkg/mcp/application/lifecycle"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// serverImpl represents the MCP server implementation using focused components.
// This delegates responsibilities to specialized components for better separation of concerns.
type serverImpl struct {
	deps             *Dependencies
	lifecycleManager *lifecycle.LifecycleManager
	bootstrapper     *bootstrap.Bootstrapper
}

// ConversationComponents represents conversation mode components
type ConversationComponents struct {
	_ bool // placeholder field
}

// NewMCPServer creates a new MCP server with the given options using Wire dependency injection.
func NewMCPServer(ctx context.Context, logger *slog.Logger, opts ...Option) (api.MCPServer, error) {
	tempDeps := &Dependencies{Config: workflow.DefaultServerConfig()}
	for _, opt := range opts {
		opt(tempDeps)
	}
	config := tempDeps.Config

	// Directory creation is now handled by the Bootstrapper component

	var server api.MCPServer
	var err error

	if tempDeps.Config.WorkspaceDir != workflow.DefaultServerConfig().WorkspaceDir ||
		tempDeps.Config.StorePath != workflow.DefaultServerConfig().StorePath ||
		tempDeps.Config.MaxSessions != workflow.DefaultServerConfig().MaxSessions {
		server, err = initializeServerWithCustomConfig(logger, config)
	} else {
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
	return initializeServer(logger)
}

// initializeServerWithCustomConfig initializes server with custom configuration
func initializeServerWithCustomConfig(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	return initializeServerWithConfig(logger, config)
}

// ServerFactory is a function type for creating servers
type ServerFactory func(logger *slog.Logger) (api.MCPServer, error)

// ServerFactoryWithConfig is a function type for creating servers with config
type ServerFactoryWithConfig func(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error)

// defaultServerFactory is set by the wire package at init time
var (
	defaultServerFactory           ServerFactory
	defaultServerFactoryWithConfig ServerFactoryWithConfig
)

// SetServerFactories sets the server factory functions
func SetServerFactories(factory ServerFactory, factoryWithConfig ServerFactoryWithConfig) {
	defaultServerFactory = factory
	defaultServerFactoryWithConfig = factoryWithConfig
}

// initializeServer wraps the Wire-generated injector with conversion to application.Dependencies
func initializeServer(logger *slog.Logger) (api.MCPServer, error) {
	if defaultServerFactory == nil {
		return nil, fmt.Errorf("server factory not initialized - Wire injection not configured")
	}
	return defaultServerFactory(logger)
}

// initializeServerWithConfig wraps the Wire-generated injector with custom config
func initializeServerWithConfig(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	if defaultServerFactoryWithConfig == nil {
		return nil, fmt.Errorf("server factory not initialized - Wire injection not configured")
	}
	return defaultServerFactoryWithConfig(logger, config)
}

// Start starts the MCP server
func (s *serverImpl) Start(ctx context.Context) error {
	return s.lifecycleManager.Start(ctx)
}

// Shutdown gracefully shuts down the server with proper context handling
func (s *serverImpl) Shutdown(ctx context.Context) error {
	return s.lifecycleManager.Shutdown(ctx)
}

// Stop stops the MCP server (implements api.MCPServer)
func (s *serverImpl) Stop(ctx context.Context) error {
	return s.lifecycleManager.Shutdown(ctx)
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
		"uptime":            s.lifecycleManager.GetUptime().String(),
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
	sessions, err := s.deps.SessionManager.List(ctx)
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
		sessions, err := s.deps.SessionManager.List(ctx)
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
	return s.bootstrapper.RegisterChatModes()
}
