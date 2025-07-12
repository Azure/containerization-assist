// Package progress provides factory functions for creating progress managers
package progress

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ManagerInterface defines the interface all progress managers must implement
type ManagerInterface interface {
	Begin(msg string)
	Update(step int, msg string, metadata map[string]interface{})
	Complete(msg string)
	Finish()

	// State getters
	GetCurrent() int
	SetCurrent(current int)
	GetTotal() int
	IsComplete() bool
	GetTraceID() string

	// Error budget methods
	RecordError(err error) bool
	RecordSuccess()
	IsCircuitOpen() bool
	GetErrorBudgetStatus() ErrorBudgetStatus
	UpdateWithErrorHandling(step int, msg string, metadata map[string]interface{}, err error) bool
}

// NewManager creates the appropriate progress manager based on environment
func NewManager(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) ManagerInterface {
	// Always use the new channel-based manager for better performance
	return NewChannelManager(ctx, req, totalSteps, logger)
}

// NewLegacyManager creates the original manager (for backward compatibility)
func NewLegacyManager(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) ManagerInterface {
	return New(ctx, req, totalSteps, logger)
}

// NewReporter creates a standalone reporter (for direct use)
func NewReporter(ctx context.Context, totalSteps int, logger *slog.Logger) Reporter {
	srv := server.ServerFromContext(ctx)

	if srv != nil {
		// Try to extract progress token from context metadata
		// This is a simplified version - in practice you'd need request metadata
		wrapper := &mcpServerWrapper{server: srv}
		return NewMCPReporter(ctx, wrapper, nil, totalSteps, logger)
	}

	return NewCLIReporter(ctx, totalSteps, logger)
}
