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

	// Fallback to CLI emitter
	if logger != nil {
		logger.Debug("Creating CLI progress emitter")
	}
	return NewCLIDirectEmitter(logger)
}

// CreateProgressEmitterWithToken creates an MCP progress emitter with an explicit token
func CreateProgressEmitterWithToken(ctx context.Context, token interface{}, logger *slog.Logger) api.ProgressEmitter {
	srv := server.ServerFromContext(ctx)
	if srv != nil && token != nil {
		return NewMCPDirectEmitter(srv, token, logger)
	}
	return NewCLIDirectEmitter(logger)
}
