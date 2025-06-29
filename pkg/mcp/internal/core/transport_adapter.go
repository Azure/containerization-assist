package core

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/transport"
)

// TransportAdapter adapts internal transport to transport.LocalTransport interface
type TransportAdapter struct {
	internal interface {
		Serve(ctx context.Context) error
		Stop(ctx context.Context) error
		Name() string
		SetHandler(handler transport.LocalRequestHandler)
	}
}

// NewTransportAdapter creates a new transport adapter
func NewTransportAdapter(t interface{}) transport.LocalTransport {
	// Type assert to ensure it has the required methods
	if transport, ok := t.(interface {
		Serve(ctx context.Context) error
		Stop(ctx context.Context) error
		Name() string
		SetHandler(handler transport.LocalRequestHandler)
	}); ok {
		return &TransportAdapter{internal: transport}
	}
	return nil
}

// Serve starts the transport and serves requests
func (ta *TransportAdapter) Serve(ctx context.Context) error {
	return ta.internal.Serve(ctx)
}

// Stop gracefully stops the transport
func (ta *TransportAdapter) Stop(ctx context.Context) error {
	return ta.internal.Stop(ctx)
}

// Name returns the transport name
func (ta *TransportAdapter) Name() string {
	return ta.internal.Name()
}

// SetHandler sets the request handler
func (ta *TransportAdapter) SetHandler(handler transport.LocalRequestHandler) {
	ta.internal.SetHandler(handler)
}

// requestHandlerAdapter adapts between different request handler types
type requestHandlerAdapter struct {
	handler transport.LocalRequestHandler
}

// HandleRequest implements transport.LocalRequestHandler
func (rha *requestHandlerAdapter) HandleRequest(ctx context.Context, req *mcp.MCPRequest) (*mcp.MCPResponse, error) {
	// Direct pass-through since both use the same types now
	response, err := rha.handler.HandleRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	// Handle nil response
	if response == nil {
		return &mcp.MCPResponse{
			Result: nil,
		}, nil
	}
	return response, nil
}
