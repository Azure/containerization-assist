package transport

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// Transport interface for transport types (local interface to avoid import cycles)
type Transport interface {
	Serve(ctx context.Context) error
	Stop() error
	Name() string
	SetHandler(handler RequestHandler)
}

// StdioTransport implements Transport for stdio communication
type StdioTransport struct {
	server       server.Server
	gomcpManager interface{} // GomcpManager interface for shutdown
	errorHandler *StdioErrorHandler
	logger       zerolog.Logger
	handler      RequestHandler
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

// Serve starts the stdio transport and blocks until context cancellation
func (s *StdioTransport) Serve(ctx context.Context) error {
	if s.handler == nil {
		return fmt.Errorf("request handler not set")
	}
	s.logger.Info().Msg("Starting stdio transport")

	// Prefer using GomcpManager if available, fallback to server
	var runFunc func() error
	if s.gomcpManager != nil {
		if mgr, ok := s.gomcpManager.(interface{ StartServer() error }); ok {
			runFunc = mgr.StartServer
		}
	}

	if runFunc == nil {
		if s.server == nil {
			return fmt.Errorf("stdio transport: neither gomcp manager nor server initialized")
		}
		runFunc = s.server.Run
	}

	// Run the server in a goroutine
	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		if err := runFunc(); err != nil {
			serverDone <- fmt.Errorf("stdio server error: %w", err)
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

// SetHandler sets the request handler for this transport
func (s *StdioTransport) SetHandler(handler RequestHandler) {
	s.handler = handler
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
	return fmt.Errorf("SendMessage should be handled by gomcp server for stdio transport")
}

// ReceiveMessage receives a message via stdio (delegated to gomcp server)
func (s *StdioTransport) ReceiveMessage() (interface{}, error) {
	// For stdio transport, message receiving is handled by the gomcp server
	// This is typically not called directly
	return nil, fmt.Errorf("ReceiveMessage should be handled by gomcp server for stdio transport")
}

// Close shuts down the transport
func (s *StdioTransport) Close() error {
	s.logger.Info().Msg("Closing stdio transport")

	// Try to shutdown gracefully using the GomcpManager
	if s.gomcpManager != nil {
		if mgr, ok := s.gomcpManager.(interface{ Shutdown(context.Context) error }); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := mgr.Shutdown(ctx); err != nil {
				s.logger.Error().Err(err).Msg("Failed to shutdown gomcp manager")
				return err
			}
		}
	}

	// Fallback: try server shutdown if available
	if s.server != nil {
		if err := s.server.Shutdown(); err != nil {
			s.logger.Error().Err(err).Msg("Failed to shutdown server")
			return err
		}
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
		return fmt.Errorf("server not initialized")
	}
	// Tool registration will be handled by the server
	return nil
}

// HandleToolError provides enhanced error handling for tool execution
func (s *StdioTransport) HandleToolError(ctx context.Context, toolName string, err error) (interface{}, error) {
	if s.errorHandler == nil {
		// Fallback to basic error handling
		return nil, fmt.Errorf("tool '%s' failed: %w", toolName, err)
	}

	// Use enhanced error handler
	startTime := time.Now()
	response, handlerErr := s.errorHandler.HandleToolError(ctx, toolName, err)
	duration := time.Since(startTime)

	// Log error metrics
	errorType := s.errorHandler.categorizeError(err)
	retryable := s.errorHandler.isRetryableError(err)
	s.errorHandler.LogErrorMetrics(toolName, errorType, duration, retryable)

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
func LogTransportInfo(transport Transport) {
	fmt.Fprintf(os.Stderr, "Starting Container Kit MCP Server on stdio transport\n")
}
