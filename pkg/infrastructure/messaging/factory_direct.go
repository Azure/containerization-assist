package messaging

import (
	"context"
	"log/slog"

	"github.com/Azure/containerization-assist/pkg/api"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CreateProgressEmitter creates the appropriate progress emitter based on context
func CreateProgressEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) api.ProgressEmitter {
	// Check if we have an MCP server in context
	srv := server.ServerFromContext(ctx)
	if srv != nil {
		// Check if the request includes a progress token
		if req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
			return NewMCPDirectEmitter(srv, req.Params.Meta.ProgressToken, logger)
		}
	}

	// Return nil if no MCP context available
	if logger != nil {
		logger.Debug("No MCP context available, progress emitter not created")
	}
	return nil
}
