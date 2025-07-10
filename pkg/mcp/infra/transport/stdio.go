package transport

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	errorcodes "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/localrivet/gomcp/server"
)

// StdioTransport implements core.CoreTransport for stdio communication
type StdioTransport struct {
	server       server.Server
	gomcpManager interface{} // GomcpManager interface for shutdown
	errorHandler *StdioErrorHandler
	logger       *slog.Logger
	handler      core.RequestHandler // Use core.RequestHandler instead of LocalRequestHandler
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport() *StdioTransport {
	// Create a default logger for now, will be updated when server is set
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil)).With(
		"transport", "stdio",
	)

	return &StdioTransport{
		logger:       logger,
		errorHandler: NewStdioErrorHandler(logger),
	}
}

// NewStdioTransportWithLogger creates a new stdio transport with a specific logger
func NewStdioTransportWithLogger(logger *slog.Logger) *StdioTransport {
	transportLogger := logger.With("transport", "stdio")

	return &StdioTransport{
		logger:       transportLogger,
		errorHandler: NewStdioErrorHandler(transportLogger),
	}
}

// NewCoreStdioTransport creates a new stdio transport that implements core.CoreTransport
func NewCoreStdioTransport(logger *slog.Logger) core.CoreTransport {
	return NewStdioTransportWithLogger(logger)
}

// Serve starts the stdio transport and blocks until context cancellation
func (s *StdioTransport) Serve(ctx context.Context) error {
	if s.handler == nil {
		systemErr := errors.SystemError(
			errorcodes.SYSTEM_ERROR,
			"Request handler not set",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		return systemErr
	}
	s.logger.Info("Starting stdio transport")

	// Use GomcpManager to start the server
	if s.gomcpManager == nil {
		systemErr := errors.SystemError(
			errorcodes.SYSTEM_ERROR,
			"STDIO transport: gomcp manager not initialized",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		return systemErr
	}

	mgr, ok := s.gomcpManager.(interface{ StartServer() error })
	if !ok {
		systemErr := errors.SystemError(
			errorcodes.SYSTEM_ERROR,
			"STDIO transport: gomcp manager does not implement StartServer",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		systemErr.Suggestions = append(systemErr.Suggestions, "Ensure gomcp manager is properly initialized")
		return systemErr
	}

	runFunc := mgr.StartServer

	// Run the server in a goroutine
	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		if err := runFunc(); err != nil {
			networkErr := errors.NetworkError(
				errorcodes.NETWORK_ERROR,
				"STDIO server error",
				err,
			)
			networkErr.Context["transport"] = "stdio"
			serverDone <- networkErr
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.logger.Info("Context cancelled, stopping stdio transport")
		return s.Close()
	case err := <-serverDone:
		if err != nil {
			s.logger.Error("Stdio server error", "error", err)
			return err
		}
		s.logger.Info("Stdio server finished")
		return nil
	}
}

// SetHandler sets the request handler for this transport (implements core.CoreTransport)
func (s *StdioTransport) SetHandler(handler core.RequestHandler) {
	s.handler = handler
}

// HandleRequest handles MCP requests directly (consolidated from RequestHandler)
// TODO: Fix MCPRequest and MCPResponse types
// func (s *StdioTransport) HandleRequest(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
// 	if s.handler == nil {
// 		return nil, errors.NewError().Messagef("no request handler configured").Build()
// 	}
// 	result, err := s.handler.HandleRequest(ctx, request)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// Type assert the result to MCPResponse
// 	response, ok := result.(*core.MCPResponse)
// 	if !ok {
// 		return nil, errors.NewError().Messagef("handler returned unexpected type: expected *core.MCPResponse, got %T", result).Build()
// 	}
// 	return response, nil
// }

// Start starts the stdio transport - alias for Serve
func (s *StdioTransport) Start() error {
	return s.Serve(context.Background())
}

// StartWithContext starts the stdio transport with context
func (s *StdioTransport) StartWithContext(ctx context.Context) error {
	return s.Serve(ctx)
}

// Stop gracefully shuts down the stdio transport (alias for Close for interface compatibility)
func (s *StdioTransport) Stop() error {
	return s.Close()
}

// StopWithContext gracefully shuts down the stdio transport with context
func (s *StdioTransport) StopWithContext(ctx context.Context) error {
	return s.Close()
}

// SendMessage sends a message via stdio (delegated to gomcp server)
func (s *StdioTransport) SendMessage(message interface{}) error {
	// For stdio transport, message sending is handled by the gomcp server
	// This is typically not called directly
	systemErr := errors.SystemError(
		errorcodes.SYSTEM_ERROR,
		"SendMessage should be handled by gomcp server for stdio transport",
		nil,
	)
	systemErr.Context["method"] = "SendMessage"
	systemErr.Context["transport"] = "stdio"
	return systemErr
}

// Send implements core.CoreTransport interface
func (s *StdioTransport) Send(ctx context.Context, message interface{}) error {
	return s.SendMessage(message)
}

// ReceiveMessage receives a message via stdio (delegated to gomcp server)
func (s *StdioTransport) ReceiveMessage() (interface{}, error) {
	// For stdio transport, message receiving is handled by the gomcp server
	// This is typically not called directly
	systemErr := errors.SystemError(
		errorcodes.SYSTEM_ERROR,
		"ReceiveMessage should be handled by gomcp server for stdio transport",
		nil,
	)
	systemErr.Context["method"] = "ReceiveMessage"
	systemErr.Context["transport"] = "stdio"
	return nil, systemErr
}

// Receive implements core.CoreTransport interface
func (s *StdioTransport) Receive(ctx context.Context) (interface{}, error) {
	return s.ReceiveMessage()
}

// Close shuts down the transport
func (s *StdioTransport) Close() error {
	s.logger.Info("Closing stdio transport")

	// Shutdown using the GomcpManager
	if s.gomcpManager == nil {
		systemErr := errors.SystemError(
			errorcodes.SYSTEM_ERROR,
			"STDIO transport: gomcp manager not initialized",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		return systemErr
	}

	mgr, ok := s.gomcpManager.(interface{ Shutdown(context.Context) error })
	if !ok {
		systemErr := errors.SystemError(
			errorcodes.SYSTEM_ERROR,
			"STDIO transport: gomcp manager does not implement Shutdown",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check gomcp manager implementation")
		return systemErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mgr.Shutdown(ctx); err != nil {
		s.logger.Error("Failed to shutdown gomcp manager", "error", err)
		return err
	}

	s.logger.Info("Stdio transport closed successfully")
	return nil
}

// Name returns the transport name
func (s *StdioTransport) Name() string {
	return "stdio"
}

// GetServer returns the underlying MCP server for tool registration
func (s *StdioTransport) GetServer() server.Server {
	return s.server
}

// SetServer sets the MCP server
func (s *StdioTransport) SetServer(srv server.Server) {
	s.server = srv
}

// SetGomcpManager sets the GomcpManager for proper shutdown
func (s *StdioTransport) SetGomcpManager(manager interface{}) {
	s.gomcpManager = manager
}

// RegisterTool is a helper to register tools with the underlying MCP server
func (s *StdioTransport) RegisterTool(name, description string, handler interface{}) error {
	if s.server == nil {
		systemErr := errors.SystemError(
			errorcodes.SYSTEM_UNAVAILABLE,
			"Server not initialized",
			nil,
		)
		systemErr.Context["component"] = "stdio_server"
		return systemErr
	}
	// Tool registration will be handled by the server
	return nil
}

// HandleToolError provides enhanced error handling for tool execution
func (s *StdioTransport) HandleToolError(ctx context.Context, toolName string, err error) (interface{}, error) {
	if s.errorHandler == nil {
		// Fallback to basic error handling
		systemErr := errors.SystemError(
			errorcodes.SYSTEM_ERROR,
			fmt.Sprintf("Tool '%s' failed", toolName),
			err,
		)
		systemErr.Context["tool"] = toolName
		systemErr.Context["component"] = "stdio_server"
		return nil, systemErr
	}

	// Use enhanced error handler
	startTime := time.Now()
	response, handlerErr := s.errorHandler.HandleToolError(ctx, toolName, err)
	duration := time.Since(startTime)

	// Log error metrics
	errorType := s.errorHandler.categorizeError(err)
	retryable := s.errorHandler.isRetryableError(err)
	s.errorHandler.LogErrorDetails(toolName, errorType, duration, retryable)

	return response, handlerErr
}

// CreateErrorResponse creates a standardized error response
func (s *StdioTransport) CreateErrorResponse(id interface{}, code int, message string, data interface{}) map[string]interface{} {
	if s.errorHandler == nil {
		// Fallback response
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]interface{}{
				"code":    code,
				"message": message,
				"data":    data,
			},
		}
	}

	return s.errorHandler.CreateErrorResponse(id, code, message, data)
}

// UpdateLogger updates the transport logger (useful when server context is available)
func (s *StdioTransport) UpdateLogger(logger *slog.Logger) {
	s.logger = logger.With("transport", "stdio")
	s.errorHandler = NewStdioErrorHandler(s.logger)
}

// GetErrorHandler returns the error handler (for testing or advanced usage)
func (s *StdioTransport) GetErrorHandler() *StdioErrorHandler {
	return s.errorHandler
}

// CreateRecoveryResponse creates a response with recovery guidance
func (s *StdioTransport) CreateRecoveryResponse(originalError error, recoverySteps, alternatives []string) interface{} {
	if s.errorHandler == nil {
		return map[string]interface{}{
			"error":   originalError.Error(),
			"message": "Error occurred but no recovery handler available",
		}
	}

	return s.errorHandler.CreateRecoveryResponse(originalError, recoverySteps, alternatives)
}

// LogTransportInfo logs transport startup information
func LogTransportInfo(transport core.CoreTransport) {
	fmt.Fprintf(os.Stderr, "Starting Container Kit MCP Server on stdio transport\n")
}
