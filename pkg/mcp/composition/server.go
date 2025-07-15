// Package composition wires the complete MCP server following the 4-layer model.
// It deliberately lives *outside* the api/ tree to keep that layer pure.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build wireinject
// +build wireinject

package composition

import (
	"log/slog"

	"github.com/google/wire"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// InitializeServer is the single entry point used by main().
func InitializeServer(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	wire.Build(ProviderSet)
	return nil, nil
}
