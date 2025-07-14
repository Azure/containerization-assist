// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/mark3labs/mcp-go/server"
)

// Transport defines the interface for MCP transport implementations
type Transport interface {
	Serve(ctx context.Context, mcpServer *server.MCPServer) error
}

// TransportType represents the type of transport
type TransportType string

const (
	TransportTypeStdio TransportType = "stdio"
	TransportTypeHTTP  TransportType = "http"
)

// TransportRegistry is a type alias for the generic registry
type TransportRegistry = registry.Registry[Transport]

// Registry holds registered transport implementations
type Registry struct {
	transports *TransportRegistry
	logger     *slog.Logger
}

// NewRegistry creates a new transport registry
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		transports: registry.New[Transport](),
		logger:     logger.With("component", "transport_registry"),
	}
}

// Register adds a transport implementation to the registry
func (r *Registry) Register(transportType TransportType, transport Transport) {
	r.transports.Add(string(transportType), transport)
	r.logger.Debug("Transport registered", "type", transportType)
}

// Start starts the specified transport
func (r *Registry) Start(ctx context.Context, transportType TransportType, mcpServer *server.MCPServer) error {
	transport, exists := r.transports.Get(string(transportType))
	if !exists {
		return fmt.Errorf("unsupported transport type: %s", transportType)
	}

	r.logger.Info("Starting transport", "type", transportType)
	return transport.Serve(ctx, mcpServer)
}
