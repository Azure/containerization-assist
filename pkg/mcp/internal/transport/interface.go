package transport

import (
	"context"
)

// Transport defines the interface for MCP transport mechanisms
type Transport interface {
	// Serve starts the transport and handles requests
	Serve(ctx context.Context, handler RequestHandler) error

	// Close gracefully shuts down the transport
	Close() error

	// Name returns the transport name for logging
	Name() string
}

// RequestHandler processes MCP requests
type RequestHandler interface {
	// HandleRequest processes an incoming MCP request
	HandleRequest(ctx context.Context, req *MCPRequest) (*MCPResponse, error)
}

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error response
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Common MCP error codes
const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603

	// Custom error codes
	ErrorCodeSessionNotFound = -32001
	ErrorCodeQuotaExceeded   = -32002
	ErrorCodeCircuitOpen     = -32003
	ErrorCodeJobNotFound     = -32004
)
