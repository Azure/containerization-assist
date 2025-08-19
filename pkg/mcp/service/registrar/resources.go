// Package registrar handles tool and prompt registration
package registrar

import (
	"log/slog"

	"github.com/Azure/containerization-assist/pkg/common/errors"
	resources "github.com/Azure/containerization-assist/pkg/mcp/infrastructure/core/resources"
	"github.com/mark3labs/mcp-go/server"
)

// ResourceRegistrar handles resource registration
type ResourceRegistrar struct {
	logger        *slog.Logger
	resourceStore *resources.Store
}

// NewResourceRegistrar creates a new resource registrar
func NewResourceRegistrar(logger *slog.Logger, store *resources.Store) *ResourceRegistrar {
	return &ResourceRegistrar{
		logger:        logger.With("component", "resource_registrar"),
		resourceStore: store,
	}
}

// RegisterAll registers all resource providers with the MCP server
func (rr *ResourceRegistrar) RegisterAll(mcpServer *server.MCPServer) error {
	rr.logger.Info("Registering resource providers")

	if err := rr.resourceStore.RegisterProviders(mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "registrar", "failed to register resource providers", err)
	}

	rr.logger.Info("All resource providers registered successfully")
	return nil
}
