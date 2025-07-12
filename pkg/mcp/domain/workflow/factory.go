// Package workflow provides progress tracking for workflows
package workflow

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewProgressTracker creates a new progress tracker for workflow operations.
func NewProgressTracker(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) *progress.Tracker {
	factory := progress.NewSinkFactory(logger)
	traceID := generateTraceID()

	return factory.CreateTrackerForWorkflow(ctx, req, totalSteps, traceID)
}
