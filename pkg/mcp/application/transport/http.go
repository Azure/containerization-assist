// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/transport/http"
	"github.com/Azure/container-kit/pkg/mcp/domain/health"
	"github.com/mark3labs/mcp-go/server"
)

// HTTPTransport handles HTTP-based MCP communication
type HTTPTransport struct {
	handler *http.Handler
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(logger *slog.Logger, port int) *HTTPTransport {
	// Create a basic health monitor for HTTP transport
	// In a production setup, this would be provided by dependency injection
	healthMonitor := &basicHealthMonitor{}
	return &HTTPTransport{
		handler: http.NewHandler(logger, port, healthMonitor),
	}
}

// basicHealthMonitor is a minimal implementation of health.Monitor for HTTP transport
type basicHealthMonitor struct{}

func (m *basicHealthMonitor) RegisterChecker(checker health.Checker) {}

func (m *basicHealthMonitor) GetHealth(ctx context.Context) health.HealthReport {
	return health.HealthReport{
		Status:     health.StatusHealthy,
		Components: make(map[string]health.ComponentHealth),
	}
}

func (m *basicHealthMonitor) GetComponentHealth(ctx context.Context, component string) (health.Status, error) {
	return health.StatusHealthy, nil
}

// Serve implements the Transport interface
func (t *HTTPTransport) Serve(ctx context.Context, mcpServer *server.MCPServer) error {
	return t.handler.Serve(ctx, mcpServer)
}

// ServeHTTP starts the HTTP transport server (deprecated - use Serve)
func (t *HTTPTransport) ServeHTTP(ctx context.Context, mcpServer *server.MCPServer) error {
	return t.Serve(ctx, mcpServer)
}
