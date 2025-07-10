package runtime

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/commands"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// RegisterAllTools registers all commands with the unified registry
func RegisterAllTools(
	registry api.ToolRegistry,
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	logger *slog.Logger,
) error {
	// Initialize commands with the unified registry
	err := commands.InitializeCommands(registry, sessionStore, sessionState, logger)
	if err != nil {
		return errors.NewError().
			Message("failed to initialize commands").
			Cause(err).
			Build()
	}
	return nil
}

// GetAllToolNames returns all registered tool names from the unified registry
func GetAllToolNames(registry api.ToolRegistry) []string {
	if registry == nil {
		return []string{}
	}
	return registry.List()
}

// GetToolCount returns the count of registered tools
func GetToolCount(registry api.ToolRegistry) int {
	if registry == nil {
		return 0
	}
	return len(registry.List())
}
