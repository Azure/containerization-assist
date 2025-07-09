package services

import "context"

// ServiceTransport defines the interface for MCP protocol transports
// ServiceTransport - Use api.Transport for the canonical interface
// This version has different method signatures than the canonical version
// Deprecated: Use api.Transport for new code
type ServiceTransport interface {
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
	CreateStdioTransport() ServiceTransport

	// CreateHTTPTransport creates an HTTP transport
	CreateHTTPTransport(config interface{}) ServiceTransport
}
