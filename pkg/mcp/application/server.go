package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// serverImpl represents the consolidated MCP server implementation
type serverImpl struct {
	config         workflow.ServerConfig
	sessionManager session.SessionManager
	resourceStore  *resources.Store
	logger         *slog.Logger
	startTime      time.Time

	mcpServer        *server.MCPServer
	isMcpInitialized bool

	shutdownMutex  sync.Mutex
	isShuttingDown bool
}

// ConversationComponents represents conversation mode components
type ConversationComponents struct {
	_ bool // placeholder field
}

// registerTools registers the single comprehensive workflow tool
func (s *serverImpl) registerTools() error {
	if s.mcpServer == nil {
		return errors.New(errors.CodeInternalError, "server", "mcp server not initialized", nil)
	}

	s.logger.Info("Registering single comprehensive workflow tool for AI-powered containerization")
	if err := workflow.RegisterWorkflowTools(s.mcpServer, s.logger); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "server", "failed to register workflow tools", err)
	}

	// Keep essential diagnostic tools
	pingTool := mcp.Tool{
		Name:        "ping",
		Description: "Simple ping tool to test MCP connectivity",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Optional message to echo back",
				},
			},
		},
	}
	s.mcpServer.AddTool(pingTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.GetArguments()
		message, _ := arguments["message"].(string)

		response := "pong"
		if message != "" {
			response = "pong: " + message
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"response":"%s","timestamp":"%s"}`, response, time.Now().Format(time.RFC3339)),
				},
			},
		}, nil
	})

	statusTool := mcp.Tool{
		Name:        "server_status",
		Description: "Get basic server status information",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"details": map[string]interface{}{
					"type":        "boolean",
					"description": "Include detailed information",
				},
			},
		},
	}
	s.mcpServer.AddTool(statusTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.GetArguments()
		details, _ := arguments["details"].(bool)

		status := struct {
			Status  string `json:"status"`
			Version string `json:"version"`
			Uptime  string `json:"uptime"`
			Details bool   `json:"details,omitempty"`
		}{
			Status:  "running",
			Version: "dev",
			Uptime:  time.Since(s.startTime).String(),
			Details: details,
		}

		statusJSON, _ := json.Marshal(status)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(statusJSON),
				},
			},
		}, nil
	})

	s.logger.Info("Workflow tools registered successfully - AI will now use complete workflows instead of atomic tools")

	// Register MCP prompts for slash commands
	promptRegistry := prompts.NewRegistry(s.mcpServer, s.logger)
	if err := promptRegistry.RegisterAll(); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "server", "failed to register prompts", err)
	}

	// Register MCP resource providers
	if err := s.resourceStore.RegisterProviders(s.mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "server", "failed to register resource providers", err)
	}

	return nil
}

// dependencies holds internal dependencies for the server
type dependencies struct {
	sessionManager session.SessionManager
	logger         *slog.Logger
}

// NewServer creates a new MCP server with the given options
// This is the primary public API for creating MCP servers
func NewServer(ctx context.Context, logger *slog.Logger, opts ...Option) (api.MCPServer, error) {
	// Build configuration from functional options
	config := workflow.DefaultServerConfig()
	for _, opt := range opts {
		opt(&config)
	}
	// Create server logger
	serverLogger := logger.With("component", "mcp-server")

	// Create internal dependencies
	sessionManager := session.NewMemorySessionManager(logger, config.SessionTTL, config.MaxSessions)
	serverLogger.Info("Created enhanced in-memory session manager",
		"default_ttl", config.SessionTTL,
		"max_sessions", config.MaxSessions)

	deps := &dependencies{
		sessionManager: sessionManager,
		logger:         serverLogger,
	}

	if config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(config.StorePath), 0o755); err != nil {
			serverLogger.Error("Failed to create storage directory", "error", err, "path", config.StorePath)
			return nil, errors.New(errors.CodeIoError, "server", fmt.Sprintf("failed to create storage directory %s", config.StorePath), err)
		}
	}

	// Validate workspace directory exists or can be created
	if config.WorkspaceDir != "" {
		if err := os.MkdirAll(config.WorkspaceDir, 0o755); err != nil {
			serverLogger.Error("Failed to create workspace directory", "error", err, "path", config.WorkspaceDir)
			return nil, errors.New(errors.CodeIoError, "server", fmt.Sprintf("failed to create workspace directory %s", config.WorkspaceDir), err)
		}
	}

	// Create resource store
	resourceStore := resources.NewStore(deps.logger)

	server := &serverImpl{
		config:         config,
		sessionManager: deps.sessionManager,
		resourceStore:  resourceStore,
		logger:         deps.logger,
		startTime:      time.Now(),
	}

	deps.logger.Info("MCP Server initialized successfully",
		"transport", config.TransportType,
		"workspace_dir", config.WorkspaceDir,
		"max_sessions", config.MaxSessions)

	return server, nil
}

// Start starts the MCP server
func (s *serverImpl) Start(ctx context.Context) error {
	s.logger.Info("Starting Container Kit MCP Server",
		"transport", s.config.TransportType,
		"workspace_dir", s.config.WorkspaceDir,
		"max_sessions", s.config.MaxSessions)

	if err := s.sessionManager.StartCleanupRoutine(ctx); err != nil {
		s.logger.Error("Failed to start cleanup routine", "error", err)
		return err
	}

	// Start resource store cleanup routine
	s.resourceStore.StartCleanupRoutine(30*time.Minute, 24*time.Hour)

	// Initialize mcp-go server directly without manager abstraction
	if !s.isMcpInitialized {
		s.logger.Info("Initializing mcp-go server")

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

		// Register tools directly
		if err := s.registerTools(); err != nil {
			return errors.New(errors.CodeToolExecutionFailed, "transport", "failed to register tools with mcp-go", err)
		}

		// Register chat modes for Copilot integration
		if err := s.RegisterChatModes(); err != nil {
			s.logger.Warn("Failed to register chat modes", "error", err)
			// Don't fail server startup for this
		}

		s.isMcpInitialized = true
		s.logger.Info("MCP-GO server initialized successfully")
	}

	// Use mcp-go server Serve method
	transportDone := make(chan error, 1)
	go func() {
		// mcp-go uses ServeStdio() method for stdio transport
		transportDone <- server.ServeStdio(s.mcpServer)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("Server stopped by context cancellation")
		return ctx.Err()
	case err := <-transportDone:
		s.logger.Error("Transport stopped with error", "error", err)
		return err
	}
}

// Shutdown gracefully shuts down the server with proper context handling
func (s *serverImpl) Shutdown(ctx context.Context) error {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()

	if s.isShuttingDown {
		return nil // Already shutting down
	}
	s.isShuttingDown = true

	s.logger.Info("Gracefully shutting down MCP Server")

	// Stop session manager with context awareness
	done := make(chan error, 1)
	go func() {
		if err := s.sessionManager.Stop(ctx); err != nil {
			s.logger.Error("Failed to stop session manager", "error", err)
			done <- err
			return
		}
		done <- nil
	}()

	// Wait for shutdown or context cancellation
	select {
	case <-ctx.Done():
		s.logger.Warn("Shutdown cancelled by context", "error", ctx.Err())
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}

	s.logger.Info("MCP Server shutdown complete")
	return nil
}

// Stop stops the MCP server (implements api.MCPServer)
func (s *serverImpl) Stop(ctx context.Context) error {
	// Use the provided context for shutdown
	return s.Shutdown(ctx)
}

// EnableConversationMode enables conversation mode (workflow-focused server - no-op)
func (s *serverImpl) EnableConversationMode(_ interface{}) error {
	s.logger.Info("Conversation mode not supported in workflow-focused server")
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
		"transport_type":    s.config.TransportType,
		"conversation_mode": s.IsConversationModeEnabled(),
	}, nil
}

// getSessionCount returns the current number of sessions
func (s *serverImpl) getSessionCount() int {
	if s.sessionManager == nil {
		return 0
	}

	ctx := context.Background()
	sessions, err := s.sessionManager.ListSessionsTyped(ctx)
	if err != nil {
		s.logger.Warn("Failed to get session count", "error", err)
		return 0
	}

	return len(sessions)
}

// GetSessionManagerStats returns session manager statistics
func (s *serverImpl) GetSessionManagerStats() (interface{}, error) {
	if s.sessionManager != nil {
		ctx := context.Background()
		sessions, err := s.sessionManager.ListSessionsTyped(ctx)
		if err != nil {
			s.logger.Warn("Failed to get session list for stats", "error", err)
			return map[string]interface{}{
				"error":        "failed to retrieve session stats",
				"max_sessions": s.config.MaxSessions,
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
			"max_sessions":    s.config.MaxSessions,
		}, nil
	}
	return map[string]interface{}{
		"error": "session manager not initialized",
	}, nil
}
