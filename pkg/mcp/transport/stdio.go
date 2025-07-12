// Package transport handles MCP transport layer concerns
package transport

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/server"
)

// StdioTransport handles stdio-based MCP communication
type StdioTransport struct {
	logger *slog.Logger
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(logger *slog.Logger) *StdioTransport {
	return &StdioTransport{
		logger: logger.With("component", "stdio_transport"),
	}
}

// ServeStdio starts the stdio transport server
func (t *StdioTransport) ServeStdio(ctx context.Context, mcpServer *server.MCPServer) error {
	t.logger.Info("Starting stdio transport")

	// Create error channel for transport
	transportDone := make(chan error, 1)

	// Run transport in goroutine
	go func() {
		// mcp-go uses ServeStdio() method for stdio transport
		transportDone <- server.ServeStdio(mcpServer)
	}()

	// Wait for context cancellation or transport error
	select {
	case <-ctx.Done():
		t.logger.Info("Stdio transport stopped by context cancellation")
		return ctx.Err()
	case err := <-transportDone:
		if err != nil {
			t.logger.Error("Stdio transport stopped with error", "error", err)
		} else {
			t.logger.Info("Stdio transport stopped gracefully")
		}
		return err
	}
}
