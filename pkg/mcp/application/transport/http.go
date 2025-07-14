// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/transport/http"
	"github.com/mark3labs/mcp-go/server"
)

// HTTPTransport handles HTTP-based MCP communication
type HTTPTransport struct {
	handler *http.Handler
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(logger *slog.Logger, port int) *HTTPTransport {
	return &HTTPTransport{
		handler: http.NewHandler(logger, port),
	}
}

// Serve implements the Transport interface
func (t *HTTPTransport) Serve(ctx context.Context, mcpServer *server.MCPServer) error {
	return t.handler.Serve(ctx, mcpServer)
}

// ServeHTTP starts the HTTP transport server (deprecated - use Serve)
func (t *HTTPTransport) ServeHTTP(ctx context.Context, mcpServer *server.MCPServer) error {
	return t.Serve(ctx, mcpServer)
}
