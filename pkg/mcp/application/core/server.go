package core

// NOTE: The following interfaces have been consolidated into ServerService
// in unified_interfaces.go for better maintainability:
// - Transport
// - ToolOrchestrator
// - TypedPipelineOperations
// - Server
// - RequestHandler
//
// Use ServerService instead of these interfaces for new implementations.

// The supporting types and concrete implementations remain in this file.

import (
	"context"
)

// Server interface for backward compatibility
// Deprecated: Use ServerService from unified_interfaces.go instead
type Server interface {
	Start(ctx context.Context) error
	Stop() error
	GetName() string
	EnableConversationMode(config ConsolidatedConversationConfig) error
	GetStats() (interface{}, error)
	Shutdown(ctx context.Context) error
	GetSessionManagerStats() (interface{}, error)
}

// RequestHandler interface for backward compatibility
// Deprecated: Use ServerService from unified_interfaces.go instead
type RequestHandler interface {
	HandleRequest(ctx context.Context, request interface{}) (interface{}, error)
}

// CoreTransport interface for backward compatibility
// Deprecated: Use TransportService from unified_interfaces.go instead
type CoreTransport interface {
	Start() error
	Stop() error
	Send(ctx context.Context, message interface{}) error
	Receive(ctx context.Context) (interface{}, error)
}
