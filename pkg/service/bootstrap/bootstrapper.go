// Package bootstrap provides server initialization and setup logic
package bootstrap

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
	"github.com/Azure/containerization-assist/pkg/service/registrar"
	"github.com/Azure/containerization-assist/pkg/service/session"
	"github.com/mark3labs/mcp-go/server"
)

// Alias for session manager interface
type OptimizedSessionManager = session.OptimizedSessionManager

// Bootstrapper handles server initialization and component registration
type Bootstrapper struct {
	logger               *slog.Logger
	config               workflow.ServerConfig
	resourceStore        *core.Store
	workflowOrchestrator workflow.WorkflowOrchestrator
	sessionManager       OptimizedSessionManager
}

// NewBootstrapper creates a new bootstrapper instance
func NewBootstrapper(
	logger *slog.Logger,
	config workflow.ServerConfig,
	resourceStore *core.Store,
	workflowOrchestrator workflow.WorkflowOrchestrator,
	sessionManager OptimizedSessionManager,
) *Bootstrapper {
	return &Bootstrapper{
		logger:               logger,
		config:               config,
		resourceStore:        resourceStore,
		workflowOrchestrator: workflowOrchestrator,
		sessionManager:       sessionManager,
	}
}

// InitializeDirectories creates necessary directories for the server
func (b *Bootstrapper) InitializeDirectories() error {
	if b.config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(b.config.StorePath), 0o755); err != nil {
			return errors.New(errors.CodeIoError, "bootstrapper", fmt.Sprintf("failed to create storage directory %s", b.config.StorePath), err)
		}
	}

	if b.config.WorkspaceDir != "" {
		if err := os.MkdirAll(b.config.WorkspaceDir, 0o755); err != nil {
			return errors.New(errors.CodeIoError, "bootstrapper", fmt.Sprintf("failed to create workspace directory %s", b.config.WorkspaceDir), err)
		}
	}

	return nil
}

// CreateMCPServer creates a new mcp-go server with capabilities
func (b *Bootstrapper) CreateMCPServer() *server.MCPServer {

	mcpServer := server.NewMCPServer(
		"containerization-assist-v2",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	if mcpServer == nil {
		return nil
	}

	// Enable sampling capability - this allows the server to request LLM completions from clients
	mcpServer.EnableSampling()
	b.logger.Info("Sampling capability enabled for MCP server")

	return mcpServer
}

// RegisterComponents registers all tools, prompts, and resources with the MCP server
func (b *Bootstrapper) RegisterComponents(mcpServer *server.MCPServer) error {
	if mcpServer == nil {
		return errors.New(errors.CodeInternalError, "bootstrapper", "mcp server not initialized", nil)
	}

	// Use the existing registrar with config
	reg := registrar.NewMCPRegistrar(b.logger, b.resourceStore, b.workflowOrchestrator, b.sessionManager, b.config)
	if err := reg.RegisterAll(mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "bootstrapper", "failed to register components", err)
	}

	return nil
}

// RegisterChatModes registers custom chat modes for Copilot integration
func (b *Bootstrapper) RegisterChatModes() error {
	return nil
}
