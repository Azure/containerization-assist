// Package registrar handles tool and prompt registration
package registrar

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/mark3labs/mcp-go/server"
)

// PromptRegistrar handles prompt registration
type PromptRegistrar struct {
	logger *slog.Logger
}

// NewPromptRegistrar creates a new prompt registrar
func NewPromptRegistrar(logger *slog.Logger) *PromptRegistrar {
	return &PromptRegistrar{
		logger: logger.With("component", "prompt_registrar"),
	}
}

// RegisterAll registers all prompts with the MCP server
func (pr *PromptRegistrar) RegisterAll(mcpServer *server.MCPServer) error {
	pr.logger.Info("Registering prompts")

	// Create and register prompts
	promptRegistry := prompts.NewRegistry(mcpServer, pr.logger)
	if err := promptRegistry.RegisterAll(); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "registrar", "failed to register prompts", err)
	}

	pr.logger.Info("All prompts registered successfully")
	return nil
}
