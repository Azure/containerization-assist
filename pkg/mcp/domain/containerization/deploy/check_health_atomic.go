package deploy

import (
	"context"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/localrivet/gomcp/server"
)

// This file now serves as a compatibility layer and main entry point.
// The actual implementation has been split into focused modules:
// - health_types.go: Type definitions and data structures
// - health_checker.go: Core health checking operations
// - health_validator.go: Health analysis and validation logic
// - health_tool.go: Main tool orchestration and interface

// NewAtomicCheckHealthTool creates a new atomic check health tool using unified session manager
// This is the main entry point that external packages should use
func NewAtomicCheckHealthTool(adapter core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) *AtomicCheckHealthTool {
	// Forward to the actual implementation in health_tool.go
	return newAtomicCheckHealthToolImplUnified(adapter, sessionManager, logger)
}

// NewAtomicCheckHealthToolWithServices creates a new atomic check health tool using service container
func NewAtomicCheckHealthToolWithServices(adapter core.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicCheckHealthTool {
	// Use focused services directly - no wrapper needed!
	// Forward to the new implementation that uses focused services
	return newAtomicCheckHealthToolImplServices(adapter, serviceContainer.SessionStore(), serviceContainer.SessionState(), logger)
}

// ExecutionContext contains common dependencies for atomic tool execution
type ExecutionContext struct {
	Adapter        core.TypedPipelineOperations
	SessionManager session.UnifiedSessionManager
	Logger         *slog.Logger
}

// Legacy compatibility functions - these delegate to the main tool implementation

// ExecuteHealthCheck provides direct function calls using unified session manager
func ExecuteHealthCheck(ctx context.Context, args AtomicCheckHealthArgs, execCtx ExecutionContext) (*AtomicCheckHealthResult, error) {
	tool := NewAtomicCheckHealthTool(execCtx.Adapter, execCtx.SessionManager, execCtx.Logger)
	return tool.ExecuteHealthCheck(ctx, args)
}

// ExecuteWithContext provides server context calls using unified session manager
func ExecuteWithContext(serverCtx *server.Context, args AtomicCheckHealthArgs, execCtx ExecutionContext) (*AtomicCheckHealthResult, error) {
	tool := NewAtomicCheckHealthTool(execCtx.Adapter, execCtx.SessionManager, execCtx.Logger)
	return tool.ExecuteWithContext(serverCtx, args)
}
