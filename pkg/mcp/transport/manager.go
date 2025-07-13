// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/server"
)

// TransportType represents the type of transport
type TransportType string

const (
	TransportTypeStdio TransportType = "stdio"
	TransportTypeHTTP  TransportType = "http"
)

// Manager handles transport lifecycle management
type Manager struct {
	logger        *slog.Logger
	transportType TransportType
	httpPort      int
}

// NewManager creates a new transport manager
func NewManager(logger *slog.Logger, transportType TransportType, httpPort int) *Manager {
	return &Manager{
		logger:        logger.With("component", "transport_manager"),
		transportType: transportType,
		httpPort:      httpPort,
	}
}

// Start starts the appropriate transport based on configuration
func (m *Manager) Start(ctx context.Context, mcpServer *server.MCPServer) error {
	m.logger.Info("Starting transport", "type", m.transportType)

	switch m.transportType {
	case TransportTypeStdio:
		transport := NewStdioTransport(m.logger)
		return transport.ServeStdio(ctx, mcpServer)

	case TransportTypeHTTP:
		transport := NewHTTPTransport(m.logger, m.httpPort)
		return transport.ServeHTTP(ctx, mcpServer)

	default:
		return fmt.Errorf("unsupported transport type: %s", m.transportType)
	}
}
