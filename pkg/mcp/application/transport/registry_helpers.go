// Package transport provides helper functions for common transport operations
package transport

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/server"
)

// StartDefault creates a new registry with default transports and starts the specified transport type.
// This is a convenience function for simple use cases.
func StartDefault(ctx context.Context, logger *slog.Logger, t TransportType, s *server.MCPServer) error {
	registry := NewRegistry(logger)

	// Register default transports
	registry.Register(TransportTypeStdio, NewStdioTransport(logger))
	registry.Register(TransportTypeHTTP, NewHTTPTransport(logger, 8080))          // Default port
	registry.Register(TransportTypeStreaming, NewStreamingTransport(logger, 100)) // Default buffer size

	return registry.Start(ctx, t, s)
}

// StartDefaultWithPort creates a new registry with default transports and starts the specified transport type.
// For HTTP transport, it uses the provided port.
func StartDefaultWithPort(ctx context.Context, logger *slog.Logger, t TransportType, s *server.MCPServer, httpPort int) error {
	registry := NewRegistry(logger)

	// Register default transports
	registry.Register(TransportTypeStdio, NewStdioTransport(logger))
	registry.Register(TransportTypeHTTP, NewHTTPTransport(logger, httpPort))
	registry.Register(TransportTypeStreaming, NewStreamingTransport(logger, 100)) // Default buffer size

	return registry.Start(ctx, t, s)
}
