package progress

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DirectProgressFactory creates progress emitters using the direct MCP approach
// This replaces the complex factory hierarchy with a single, simple factory
type DirectProgressFactory struct {
	logger *slog.Logger
}

// NewDirectProgressFactory creates a new direct progress factory
func NewDirectProgressFactory(logger *slog.Logger) *DirectProgressFactory {
	return &DirectProgressFactory{
		logger: logger.With("component", "direct_progress_factory"),
	}
}

// CreateEmitter creates the appropriate progress emitter based on context
// It implements workflow.ProgressEmitterFactory interface
func (f *DirectProgressFactory) CreateEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	// Check if we have an MCP server in context
	srv := server.ServerFromContext(ctx)
	if srv != nil {
		// Check if the request includes a progress token
		if req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
			f.logger.Debug("Creating MCP direct progress emitter",
				"has_server", true,
				"has_token", true,
				"total_steps", totalSteps,
			)
			return NewMCPDirectEmitter(srv, req.Params.Meta.ProgressToken, f.logger)
		}

		f.logger.Debug("MCP server found but no progress token provided")
	}

	// Fallback to CLI emitter
	f.logger.Debug("Creating CLI progress emitter",
		"has_server", srv != nil,
		"total_steps", totalSteps,
	)
	return NewCLIDirectEmitter(f.logger)
}

// CreateEmitterWithToken creates an MCP progress emitter with an explicit token
// This is useful for testing or when the token is obtained separately
func (f *DirectProgressFactory) CreateEmitterWithToken(ctx context.Context, token interface{}) api.ProgressEmitter {
	srv := server.ServerFromContext(ctx)
	if srv != nil && token != nil {
		return NewMCPDirectEmitter(srv, token, f.logger)
	}
	return NewCLIDirectEmitter(f.logger)
}

// Ensure DirectProgressFactory implements workflow.ProgressEmitterFactory
var _ workflow.ProgressEmitterFactory = (*DirectProgressFactory)(nil)
