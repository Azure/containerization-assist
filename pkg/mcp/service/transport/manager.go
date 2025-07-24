// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/service/registry"
	"github.com/mark3labs/mcp-go/server"
)

// Transport defines the interface for MCP transport implementations
type Transport interface {
	Serve(ctx context.Context, mcpServer *server.MCPServer) error
}

// TransportType represents the type of transport
type TransportType string

const (
	TransportTypeStdio     TransportType = "stdio"
	TransportTypeHTTP      TransportType = "http"
	TransportTypeStreaming TransportType = "streaming"
)

// ErrUnsupportedTransport is returned when an unsupported transport type is requested
var ErrUnsupportedTransport = fmt.Errorf("unsupported transport type")

// Registry holds registered transport implementations
type Registry struct {
	transports *registry.Registry[Transport]
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
	r.logger.Debug("Transport registered",
		slog.String("type", string(transportType)))
}

// Start starts the specified transport
func (r *Registry) Start(ctx context.Context, transportType TransportType, mcpServer *server.MCPServer) error {
	transport, exists := r.transports.Get(string(transportType))
	if !exists {
		r.logger.Error("Unsupported transport type requested",
			slog.String("transport_type", string(transportType)))
		return errors.New(
			errors.CodeInvalidParameter,
			"transport",
			fmt.Sprintf("unsupported transport type: %s", transportType),
			ErrUnsupportedTransport,
		)
	}

	r.logger.Info("Starting transport", slog.String("type", string(transportType)))

	if err := transport.Serve(ctx, mcpServer); err != nil {
		// Don't wrap context cancellation - it's expected behavior
		if err == context.Canceled || err == context.DeadlineExceeded {
			r.logger.Debug("Transport stopped due to context cancellation",
				slog.String("transport_type", string(transportType)))
			return err
		}

		r.logger.Error("Transport failed to start",
			slog.String("transport_type", string(transportType)),
			slog.String("error", err.Error()))
		return errors.New(
			errors.CodeOperationFailed,
			"transport",
			fmt.Sprintf("failed to start %s transport", transportType),
			err,
		)
	}

	return nil
}
