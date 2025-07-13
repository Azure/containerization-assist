package progress

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SinkFactory creates appropriate progress sinks based on context.
type SinkFactory struct {
	logger *slog.Logger
}

// NewSinkFactory creates a new sink factory.
func NewSinkFactory(logger *slog.Logger) *SinkFactory {
	return &SinkFactory{
		logger: logger,
	}
}

// CreateSink creates an appropriate sink based on the context.
func (f *SinkFactory) CreateSink(ctx context.Context, req *mcp.CallToolRequest) progress.Sink {
	// Try to get MCP server from context
	if srv := server.ServerFromContext(ctx); srv != nil {
		// Check if we have a progress token (MCP client wants progress updates)
		if req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
			f.logger.Debug("Creating MCP sink with progress token")
			return NewMCPSink(srv, req.Params.Meta.ProgressToken, f.logger)
		}
	}

	// Fallback to CLI sink
	f.logger.Debug("Creating CLI sink")
	return NewCLISink(f.logger)
}

// CreateTrackerForWorkflow creates a configured tracker for workflow operations.
func (f *SinkFactory) CreateTrackerForWorkflow(
	ctx context.Context,
	req *mcp.CallToolRequest,
	totalSteps int,
	traceID string,
) *progress.Tracker {
	sink := f.CreateSink(ctx, req)

	opts := []progress.Option{
		progress.WithTraceID(traceID),
		progress.WithHeartbeat(2 * time.Second), // More responsive for AI clients
		progress.WithThrottle(100 * time.Millisecond),
	}

	return progress.NewTracker(ctx, totalSteps, sink, opts...)
}

// CreateSubTracker creates a sub-tracker for detailed step progress.
func (f *SinkFactory) CreateSubTracker(
	ctx context.Context,
	req *mcp.CallToolRequest,
	totalSubSteps int,
	traceID string,
	stepName string,
) *progress.Tracker {
	sink := f.CreateSink(ctx, req)

	opts := []progress.Option{
		progress.WithTraceID(traceID),
		progress.WithHeartbeat(1 * time.Second), // Even more responsive for sub-steps
		progress.WithThrottle(50 * time.Millisecond),
	}

	tracker := progress.NewTracker(ctx, totalSubSteps, sink, opts...)
	return tracker
}

// NewProgressTracker creates a new progress tracker for workflow operations.
// This replaces the duplicate function in domain/workflow/factory.go
func NewProgressTracker(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) *progress.Tracker {
	factory := NewSinkFactory(logger)
	traceID := generateTraceID()

	return factory.CreateTrackerForWorkflow(ctx, req, totalSteps, traceID)
}

// generateTraceID creates a simple trace ID for correlation
func generateTraceID() string {
	return fmt.Sprintf("trace-%d", time.Now().UnixNano())
}
