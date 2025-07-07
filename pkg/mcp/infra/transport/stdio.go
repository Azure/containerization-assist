package transport

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// StdioTransport implements core.Transport for stdio communication
type StdioTransport struct {
	server       server.Server
	gomcpManager interface{} // GomcpManager interface for shutdown
	errorHandler *StdioErrorHandler
	logger       zerolog.Logger
	handler      core.RequestHandler // Use core.RequestHandler instead of LocalRequestHandler
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport() *StdioTransport {
	// Create a default logger for now, will be updated when server is set
	logger := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("transport", "stdio").
		Logger()

	return &StdioTransport{
		logger:       logger,
		errorHandler: NewStdioErrorHandler(logger),
	}
}

// NewStdioTransportWithLogger creates a new stdio transport with a specific logger
func NewStdioTransportWithLogger(logger zerolog.Logger) *StdioTransport {
	transportLogger := logger.With().Str("transport", "stdio").Logger()

	return &StdioTransport{
		logger:       transportLogger,
		errorHandler: NewStdioErrorHandler(transportLogger),
	}
}

// NewCoreStdioTransport creates a new stdio transport that implements core.Transport
func NewCoreStdioTransport(logger zerolog.Logger) core.Transport {
	return NewStdioTransportWithLogger(logger)
}

// Serve starts the stdio transport and blocks until context cancellation
func (s *StdioTransport) Serve(ctx context.Context) error {
	if s.handler == nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Request handler not set",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		return systemErr
	}
	s.logger.Info().Msg("Starting stdio transport")

	// Use GomcpManager to start the server
	if s.gomcpManager == nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"STDIO transport: gomcp manager not initialized",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		return systemErr
	}

	mgr, ok := s.gomcpManager.(interface{ StartServer() error })
	if !ok {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
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
				codes.NETWORK_ERROR,
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
		s.logger.Info().Msg("Context cancelled, stopping stdio transport")
		return s.Close()
	case err := <-serverDone:
		if err != nil {
			s.logger.Error().Err(err).Msg("Stdio server error")
			return err
		}
		s.logger.Info().Msg("Stdio server finished")
		return nil
	}
}

// SetHandler sets the request handler for this transport (implements core.Transport)
func (s *StdioTransport) SetHandler(handler core.RequestHandler) {
	s.handler = handler
}

// HandleRequest handles MCP requests directly (consolidated from RequestHandler)
func (s *StdioTransport) HandleRequest(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	if s.handler == nil {
		return nil, errors.NewError().Messagef("no request handler configured").Build()
	}
	return s.handler.HandleRequest(ctx, request)
}

// Start starts the stdio transport - alias for Serve
func (s *StdioTransport) Start(ctx context.Context) error {
	return s.Serve(ctx)
}

// Stop gracefully shuts down the stdio transport (alias for Close for interface compatibility)
func (s *StdioTransport) Stop(ctx context.Context) error {
	return s.Close()
}

// SendMessage sends a message via stdio (delegated to gomcp server)
func (s *StdioTransport) SendMessage(message interface{}) error {
	// For stdio transport, message sending is handled by the gomcp server
	// This is typically not called directly
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		"SendMessage should be handled by gomcp server for stdio transport",
		nil,
	)
	systemErr.Context["method"] = "SendMessage"
	systemErr.Context["transport"] = "stdio"
	return systemErr
}

// ReceiveMessage receives a message via stdio (delegated to gomcp server)
func (s *StdioTransport) ReceiveMessage() (interface{}, error) {
	// For stdio transport, message receiving is handled by the gomcp server
	// This is typically not called directly
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		"ReceiveMessage should be handled by gomcp server for stdio transport",
		nil,
	)
	systemErr.Context["method"] = "ReceiveMessage"
	systemErr.Context["transport"] = "stdio"
	return nil, systemErr
}

// Close shuts down the transport
func (s *StdioTransport) Close() error {
	s.logger.Info().Msg("Closing stdio transport")

	// Shutdown using the GomcpManager
	if s.gomcpManager == nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"STDIO transport: gomcp manager not initialized",
			nil,
		)
		systemErr.Context["component"] = "stdio_transport"
		return systemErr
	}

	mgr, ok := s.gomcpManager.(interface{ Shutdown(context.Context) error })
	if !ok {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
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
		s.logger.Error().Err(err).Msg("Failed to shutdown gomcp manager")
		return err
	}

	s.logger.Info().Msg("Stdio transport closed successfully")
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
			codes.SYSTEM_UNAVAILABLE,
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
			codes.SYSTEM_ERROR,
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
func (s *StdioTransport) UpdateLogger(logger zerolog.Logger) {
	s.logger = logger.With().Str("transport", "stdio").Logger()
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
func LogTransportInfo(transport core.Transport) {
	fmt.Fprintf(os.Stderr, "Starting Container Kit MCP Server on stdio transport\n")
}
