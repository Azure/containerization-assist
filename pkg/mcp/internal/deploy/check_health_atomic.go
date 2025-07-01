package deploy

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// This file now serves as a compatibility layer and main entry point.
// The actual implementation has been split into focused modules:
// - health_types.go: Type definitions and data structures
// - health_checker.go: Core health checking operations
// - health_validator.go: Health analysis and validation logic
// - health_tool.go: Main tool orchestration and interface

// NewAtomicCheckHealthTool creates a new atomic check health tool
// This is the main entry point that external packages should use
func NewAtomicCheckHealthTool(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicCheckHealthTool {
	// Forward to the actual implementation in health_tool.go
	return newAtomicCheckHealthToolImpl(adapter, sessionManager, logger)
}

// Legacy compatibility functions - these delegate to the main tool implementation

// ExecuteHealthCheck provides backward compatibility for direct function calls
func ExecuteHealthCheck(ctx context.Context, args AtomicCheckHealthArgs, adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) (*AtomicCheckHealthResult, error) {
	tool := NewAtomicCheckHealthTool(adapter, sessionManager, logger)
	return tool.ExecuteHealthCheck(ctx, args)
}

// ExecuteWithContext provides backward compatibility for server context calls
func ExecuteWithContext(serverCtx *server.Context, args AtomicCheckHealthArgs, adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) (*AtomicCheckHealthResult, error) {
	tool := NewAtomicCheckHealthTool(adapter, sessionManager, logger)
	return tool.ExecuteWithContext(serverCtx, args)
}
