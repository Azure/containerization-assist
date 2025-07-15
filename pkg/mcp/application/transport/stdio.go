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

// Serve implements the Transport interface
func (t *StdioTransport) Serve(ctx context.Context, mcpServer *server.MCPServer) error {
	t.logger.Info("Starting stdio transport")

	// Since ServeStdio blocks and doesn't respect context cancellation,
	// we can log when context is done but can't forcefully stop ServeStdio
	go func() {
		<-ctx.Done()
		t.logger.Info("Context cancelled, but ServeStdio must be stopped externally")
	}()

	// ServeStdio blocks until EOF or error
	err := server.ServeStdio(mcpServer)

	if err != nil {
		t.logger.Error("Stdio transport stopped with error", "error", err)
	} else {
		t.logger.Info("Stdio transport stopped gracefully")
	}

	return err
}
