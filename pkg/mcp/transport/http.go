// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// HTTPTransport handles HTTP-based MCP communication
type HTTPTransport struct {
	logger *slog.Logger
	port   int
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(logger *slog.Logger, port int) *HTTPTransport {
	if port == 0 {
		port = 8080 // Default port
	}
	return &HTTPTransport{
		logger: logger.With("component", "http_transport"),
		port:   port,
	}
}

// ServeHTTP starts the HTTP transport server
func (t *HTTPTransport) ServeHTTP(ctx context.Context, mcpServer *server.MCPServer) error {
	t.logger.Info("Starting HTTP transport", "port", t.port)

	// Create HTTP server
	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%d", t.port),
		// Handler would be set up here based on mcp-go HTTP support
		// This is a placeholder as mcp-go primarily supports stdio
	}

	// Create error channel for transport
	transportDone := make(chan error, 1)

	// Run transport in goroutine
	go func() {
		transportDone <- httpServer.ListenAndServe()
	}()

	// Wait for context cancellation or transport error
	select {
	case <-ctx.Done():
		t.logger.Info("Shutting down HTTP transport")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-transportDone:
		if err != nil && err != http.ErrServerClosed {
			t.logger.Error("HTTP transport stopped with error", "error", err)
		} else {
			t.logger.Info("HTTP transport stopped gracefully")
		}
		return err
	}
}
