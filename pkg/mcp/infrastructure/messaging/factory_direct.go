package messaging

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
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
			logger.Debug("Creating MCP direct progress emitter",
				"has_server", true,
				"has_token", true,
				"total_steps", totalSteps,
			)
			return NewMCPDirectEmitter(srv, req.Params.Meta.ProgressToken, logger)
		}

		logger.Debug("MCP server found but no progress token provided")
	}

	// Fallback to CLI emitter
	logger.Debug("Creating CLI progress emitter",
		"has_server", srv != nil,
		"total_steps", totalSteps,
	)
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
