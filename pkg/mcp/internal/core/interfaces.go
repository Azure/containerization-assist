package core

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/internal/transport"
)

// InternalTransport is a local interface to avoid import cycles with pkg/mcp
// This interface matches mcp.Transport
type InternalTransport interface {
	// Serve starts the transport and serves requests
	Serve(ctx context.Context) error

	// Stop gracefully stops the transport
	Stop(ctx context.Context) error

	// SetHandler sets the request handler
	SetHandler(handler transport.LocalRequestHandler)
}

// InternalRequestHandler is a local interface to avoid import cycles with pkg/mcp
// This interface matches mcp.RequestHandler
type InternalRequestHandler interface {
	// HandleRequest processes an MCP request and returns a response
	HandleRequest(ctx context.Context, request interface{}) (interface{}, error)
}
