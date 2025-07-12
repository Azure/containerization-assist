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
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/mark3labs/mcp-go/mcp"
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

// registerTools registers the single comprehensive workflow tool
func (s *serverImpl) registerTools() error {
	if s.mcpServer == nil {
		return errors.New(errors.CodeInternalError, "server", "mcp server not initialized", nil)
	}

	s.deps.Logger.Info("Registering single comprehensive workflow tool for AI-powered containerization")
	if err := workflow.RegisterWorkflowTools(s.mcpServer, s.deps.Logger); err != nil {
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

	s.deps.Logger.Info("Workflow tools registered successfully - AI will now use complete workflows instead of atomic tools")

	// Register MCP prompts for slash commands
	promptRegistry := prompts.NewRegistry(s.mcpServer, s.deps.Logger)
	if err := promptRegistry.RegisterAll(); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "server", "failed to register prompts", err)
	}

	// Register MCP resource providers
	if err := s.deps.ResourceStore.RegisterProviders(s.mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "server", "failed to register resource providers", err)
	}

	return nil
}

// NewMCPServer creates a new MCP server with the given options using the new functional options pattern.
// This is the primary public API for creating MCP servers
func NewMCPServer(ctx context.Context, logger *slog.Logger, opts ...Option) (api.MCPServer, error) {
	// Use default configuration and provided logger
	config := workflow.DefaultServerConfig()

	// Create bootstrap options with logger and config
	bootstrapOpts := []Option{
		WithLogger(logger),
		WithConfig(config),
	}

	// Add any additional options provided
	bootstrapOpts = append(bootstrapOpts, opts...)

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

	// Use new functional options pattern to create server
	server := NewServer(bootstrapOpts...)

	server.deps.Logger.Info("MCP Server initialized successfully",
		"transport", config.TransportType,
		"workspace_dir", config.WorkspaceDir,
		"max_sessions", config.MaxSessions)

	return server, nil
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

		// Register tools directly
		if err := s.registerTools(); err != nil {
			return errors.New(errors.CodeToolExecutionFailed, "transport", "failed to register tools with mcp-go", err)
		}

		// Register chat modes for Copilot integration
		if err := s.RegisterChatModes(); err != nil {
			s.deps.Logger.Warn("Failed to register chat modes", "error", err)
			// Don't fail server startup for this
		}

		s.isMcpInitialized = true
		s.deps.Logger.Info("MCP-GO server initialized successfully")
	}

	// Use mcp-go server Serve method
	transportDone := make(chan error, 1)
	go func() {
		// mcp-go uses ServeStdio() method for stdio transport
		transportDone <- server.ServeStdio(s.mcpServer)
	}()

	select {
	case <-ctx.Done():
		s.deps.Logger.Info("Server stopped by context cancellation")
		return ctx.Err()
	case err := <-transportDone:
		s.deps.Logger.Error("Transport stopped with error", "error", err)
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
