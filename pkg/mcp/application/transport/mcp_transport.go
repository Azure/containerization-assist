// Package transport handles MCP protocol transport concerns
package transport

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPTransport defines the interface for MCP protocol transport handling
type MCPTransport interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterTool(tool mcp.Tool, handler server.ToolHandlerFunc) error
	RegisterPrompt(prompt mcp.Prompt, handler server.PromptHandlerFunc) error
	RegisterResource(resource mcp.Resource, handler server.ResourceHandlerFunc) error
	IsInitialized() bool
}

// StdioTransport implements MCPTransport using stdio
type StdioTransport struct {
	mcpServer     *server.MCPServer
	logger        *slog.Logger
	isInitialized bool
	serverName    string
	version       string
}

// NewStdioTransport creates a new stdio-based MCP transport
func NewStdioTransport(serverName, version string, logger *slog.Logger) *StdioTransport {
	return &StdioTransport{
		serverName: serverName,
		version:    version,
		logger:     logger.With("component", "mcp_transport"),
	}
}

// Initialize creates and configures the MCP server
func (t *StdioTransport) Initialize() error {
	if t.isInitialized {
		return nil
	}

	t.logger.Info("Initializing MCP stdio transport",
		"server_name", t.serverName,
		"version", t.version)

	// Create mcp-go server with capabilities
	t.mcpServer = server.NewMCPServer(
		t.serverName,
		t.version,
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	if t.mcpServer == nil {
		return fmt.Errorf("failed to create MCP server")
	}

	t.isInitialized = true
	t.logger.Info("MCP transport initialized successfully")
	return nil
}

// Start starts the MCP transport
func (t *StdioTransport) Start(ctx context.Context) error {
	if !t.isInitialized {
		if err := t.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize transport: %w", err)
		}
	}

	t.logger.Info("Starting MCP stdio transport")

	// Run the stdio server
	return server.ServeStdio(t.mcpServer)
}

// Stop stops the MCP transport
func (t *StdioTransport) Stop(ctx context.Context) error {
	t.logger.Info("Stopping MCP transport")
	// mcp-go doesn't expose a stop method, so we rely on context cancellation
	return nil
}

// RegisterTool registers a tool with the MCP server
func (t *StdioTransport) RegisterTool(tool mcp.Tool, handler server.ToolHandlerFunc) error {
	if !t.isInitialized {
		if err := t.Initialize(); err != nil {
			return fmt.Errorf("transport not initialized: %w", err)
		}
	}

	t.mcpServer.AddTool(tool, handler)
	t.logger.Debug("Registered tool", "name", tool.Name)
	return nil
}

// RegisterPrompt registers a prompt with the MCP server
func (t *StdioTransport) RegisterPrompt(prompt mcp.Prompt, handler server.PromptHandlerFunc) error {
	if !t.isInitialized {
		if err := t.Initialize(); err != nil {
			return fmt.Errorf("transport not initialized: %w", err)
		}
	}

	t.mcpServer.AddPrompt(prompt, handler)
	t.logger.Debug("Registered prompt", "name", prompt.Name)
	return nil
}

// RegisterResource registers a resource with the MCP server
func (t *StdioTransport) RegisterResource(resource mcp.Resource, handler server.ResourceHandlerFunc) error {
	if !t.isInitialized {
		if err := t.Initialize(); err != nil {
			return fmt.Errorf("transport not initialized: %w", err)
		}
	}

	t.mcpServer.AddResource(resource, handler)
	t.logger.Debug("Registered resource", "uri", resource.URI)
	return nil
}

// IsInitialized returns whether the transport is initialized
func (t *StdioTransport) IsInitialized() bool {
	return t.isInitialized
}

// GetMCPServer returns the underlying MCP server (for legacy compatibility)
func (t *StdioTransport) GetMCPServer() *server.MCPServer {
	return t.mcpServer
}
