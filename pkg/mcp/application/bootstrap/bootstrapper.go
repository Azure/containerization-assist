// Package bootstrap provides server initialization and setup logic
package bootstrap

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/application/registrar"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/server"
)

// Bootstrapper handles server initialization and component registration
type Bootstrapper struct {
	logger               *slog.Logger
	config               workflow.ServerConfig
	resourceStore        domainresources.Store
	workflowOrchestrator workflow.WorkflowOrchestrator
}

// NewBootstrapper creates a new bootstrapper instance
func NewBootstrapper(
	logger *slog.Logger,
	config workflow.ServerConfig,
	resourceStore domainresources.Store,
	workflowOrchestrator workflow.WorkflowOrchestrator,
) *Bootstrapper {
	return &Bootstrapper{
		logger:               logger,
		config:               config,
		resourceStore:        resourceStore,
		workflowOrchestrator: workflowOrchestrator,
	}
}

// InitializeDirectories creates necessary directories for the server
func (b *Bootstrapper) InitializeDirectories() error {
	if b.config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(b.config.StorePath), 0o755); err != nil {
			b.logger.Error("Failed to create storage directory", "error", err, "path", b.config.StorePath)
			return errors.New(errors.CodeIoError, "bootstrapper", fmt.Sprintf("failed to create storage directory %s", b.config.StorePath), err)
		}
	}

	if b.config.WorkspaceDir != "" {
		if err := os.MkdirAll(b.config.WorkspaceDir, 0o755); err != nil {
			b.logger.Error("Failed to create workspace directory", "error", err, "path", b.config.WorkspaceDir)
			return errors.New(errors.CodeIoError, "bootstrapper", fmt.Sprintf("failed to create workspace directory %s", b.config.WorkspaceDir), err)
		}
	}

	return nil
}

// CreateMCPServer creates a new mcp-go server with capabilities
func (b *Bootstrapper) CreateMCPServer() *server.MCPServer {
	b.logger.Info("Creating mcp-go server with capabilities")

	mcpServer := server.NewMCPServer(
		"container-kit-mcp",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	if mcpServer == nil {
		b.logger.Error("Failed to create mcp-go server")
		return nil
	}

	b.logger.Info("MCP server created successfully")
	return mcpServer
}

// RegisterComponents registers all tools, prompts, and resources with the MCP server
func (b *Bootstrapper) RegisterComponents(mcpServer *server.MCPServer) error {
	if mcpServer == nil {
		return errors.New(errors.CodeInternalError, "bootstrapper", "mcp server not initialized", nil)
	}

	// Use the existing registrar
	reg := registrar.NewMCPRegistrar(b.logger, b.resourceStore, b.workflowOrchestrator)
	if err := reg.RegisterAll(mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "bootstrapper", "failed to register components", err)
	}

	b.logger.Info("All components registered successfully")
	return nil
}

// RegisterChatModes registers custom chat modes for Copilot integration
func (b *Bootstrapper) RegisterChatModes() error {
	b.logger.Info("Chat mode support enabled via standard MCP protocol",
		"available_tools", GetChatModeFunctions())
	return nil
}

// GetChatModeFunctions returns the function names available in chat mode
func GetChatModeFunctions() []string {
	return []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"security_scan",
		"tag_image",
		"push_image",
		"generate_manifests",
		"setup_cluster",
		"deploy_application",
		"verify_deployment",
	}
}
