package services

import "context"

// Transport defines the interface for MCP protocol transports
type Transport interface {
	// Start starts the transport
	Start(ctx context.Context) error

	// Stop stops the transport
	Stop() error

	// SetHandler sets the request handler
	SetHandler(handler interface{})
}

// TransportFactory creates transport instances
type TransportFactory interface {
	// CreateStdioTransport creates a stdio transport
	CreateStdioTransport() Transport

	// CreateHTTPTransport creates an HTTP transport
	CreateHTTPTransport(config interface{}) Transport
}
