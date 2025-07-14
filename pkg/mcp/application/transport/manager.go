// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

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

// Registry holds registered transport implementations
type Registry struct {
	transports map[TransportType]Transport
	logger     *slog.Logger
	mu         sync.RWMutex // Protects transports map
}

// NewRegistry creates a new transport registry
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		transports: make(map[TransportType]Transport),
		logger:     logger.With("component", "transport_registry"),
	}
}

// Register adds a transport implementation to the registry
func (r *Registry) Register(transportType TransportType, transport Transport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transports[transportType] = transport
	r.logger.Debug("Transport registered", "type", transportType)
}

// Start starts the specified transport
func (r *Registry) Start(ctx context.Context, transportType TransportType, mcpServer *server.MCPServer) error {
	r.mu.RLock()
	transport, exists := r.transports[transportType]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unsupported transport type: %s", transportType)
	}

	r.logger.Info("Starting transport", "type", transportType)
	return transport.Serve(ctx, mcpServer)
}
