// Package workflow provides shared types and utilities for workflow management
package workflow

import (
	"context"

	"github.com/mark3labs/mcp-go/server"
)

// GetMCPServer returns the *server.MCPServer stored in ctx (or nil).
func GetMCPServer(ctx context.Context) *server.MCPServer {
	return server.ServerFromContext(ctx)
}

// Notify forwards a JSON-serializable payload to the connected client.
// It no-ops gracefully when no server is present.
func Notify(ctx context.Context, method string, params map[string]any) error {
	if srv := server.ServerFromContext(ctx); srv != nil {
		return srv.SendNotificationToClient(ctx, method, params)
	}
	return nil
}
