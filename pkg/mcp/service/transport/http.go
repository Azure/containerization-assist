// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability"
	"github.com/Azure/container-kit/pkg/mcp/service/transport/http"
	"github.com/mark3labs/mcp-go/server"
)

// HTTPTransport handles HTTP-based MCP communication
type HTTPTransport struct {
	handler *http.Handler
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(logger *slog.Logger, port int) *HTTPTransport {
	// Create a proper health monitor
	monitor := observability.NewMonitor(logger)
	healthMonitor := observability.NewHealthMonitorAdapter(monitor)
	return &HTTPTransport{
		handler: http.NewHandler(logger, port, healthMonitor),
	}
}

// Serve implements the Transport interface
func (t *HTTPTransport) Serve(ctx context.Context, mcpServer *server.MCPServer) error {
	return t.handler.Serve(ctx, mcpServer)
}
