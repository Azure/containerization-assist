package adapters

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/infra/transport"
)

// TransportFactoryImpl implements the TransportFactory interface
type TransportFactoryImpl struct{}

// NewTransportFactory creates a new transport factory
func NewTransportFactory() services.TransportFactory {
	return &TransportFactoryImpl{}
}

// CreateStdioTransport creates a stdio transport
func (f *TransportFactoryImpl) CreateStdioTransport() services.Transport {
	return &transportAdapter{
		transport: transport.NewStdioTransport(),
	}
}

// CreateHTTPTransport creates an HTTP transport
func (f *TransportFactoryImpl) CreateHTTPTransport(config interface{}) services.Transport {
	httpConfig, ok := config.(transport.HTTPTransportConfig)
	if !ok {
		// Default config if type assertion fails
		httpConfig = transport.HTTPTransportConfig{
			Address: ":8080",
		}
	}
	return &transportAdapter{
		transport: transport.NewHTTPTransport(httpConfig),
	}
}

// transportAdapter adapts the concrete transport to the interface
type transportAdapter struct {
	transport interface{}
}

// Start starts the transport
func (a *transportAdapter) Start(ctx context.Context) error {
	if starter, ok := a.transport.(interface{ Start(context.Context) error }); ok {
		return starter.Start(ctx)
	}
	return nil
}

// Stop stops the transport
func (a *transportAdapter) Stop() error {
	if stopper, ok := a.transport.(interface{ Stop() error }); ok {
		return stopper.Stop()
	}
	return nil
}

// SetHandler sets the request handler
func (a *transportAdapter) SetHandler(handler interface{}) {
	if setter, ok := a.transport.(interface{ SetHandler(interface{}) }); ok {
		setter.SetHandler(handler)
	}
}
