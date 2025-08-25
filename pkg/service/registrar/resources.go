// Package registrar handles tool and prompt registration
package registrar

import (
	"log/slog"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
	"github.com/mark3labs/mcp-go/server"
)

// ResourceRegistrar handles resource registration
type ResourceRegistrar struct {
	resourceStore *core.Store
}

// NewResourceRegistrar creates a new resource registrar
func NewResourceRegistrar(logger *slog.Logger, store *core.Store) *ResourceRegistrar {
	return &ResourceRegistrar{
		resourceStore: store,
	}
}

// RegisterAll registers all resource providers with the MCP server
func (rr *ResourceRegistrar) RegisterAll(mcpServer *server.MCPServer) error {

	if err := rr.resourceStore.RegisterProviders(mcpServer); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "registrar", "failed to register resource providers", err)
	}

	return nil
}
