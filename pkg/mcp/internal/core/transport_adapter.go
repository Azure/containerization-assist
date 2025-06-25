package core

import (
	"context"

	"github.com/Azure/container-copilot/pkg/mcp/internal/transport"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// TransportAdapter adapts internal transport to mcptypes.Transport interface
type TransportAdapter struct {
	internal interface {
		Serve(ctx context.Context) error
		Stop(ctx context.Context) error
		Name() string
		SetHandler(handler transport.LocalRequestHandler)
	}
}

// NewTransportAdapter creates a new transport adapter
func NewTransportAdapter(t interface{}) InternalTransport {
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
func (ta *TransportAdapter) SetHandler(handler interface{}) {
	// Type assert to the expected handler type
	if h, ok := handler.(transport.LocalRequestHandler); ok {
		ta.internal.SetHandler(h)
	} else if h, ok := handler.(InternalRequestHandler); ok {
		// Wrap the InternalRequestHandler to LocalRequestHandler
		ta.internal.SetHandler(&requestHandlerAdapter{handler: h})
	}
}

// requestHandlerAdapter adapts InternalRequestHandler to transport.LocalRequestHandler
type requestHandlerAdapter struct {
	handler InternalRequestHandler
}

// HandleRequest implements transport.LocalRequestHandler
func (rha *requestHandlerAdapter) HandleRequest(ctx context.Context, req *mcptypes.MCPRequest) (*mcptypes.MCPResponse, error) {
	// Call the wrapped handler
	result, err := rha.handler.HandleRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Type assert the result
	if resp, ok := result.(*mcptypes.MCPResponse); ok {
		return resp, nil
	}

	// If not already an MCPResponse, wrap it
	return &mcptypes.MCPResponse{
		Result: result,
	}, nil
}
