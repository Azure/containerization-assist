package progress

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Ensure SinkFactory implements the domain ProgressTrackerFactory interface
var _ interface {
	CreateTracker(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) *progress.Tracker
} = (*SinkFactory)(nil)

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

// CreateTracker implements the domain ProgressTrackerFactory interface
func (f *SinkFactory) CreateTracker(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) *progress.Tracker {
	traceID := generateTraceID()
	return f.CreateTrackerForWorkflow(ctx, req, totalSteps, traceID)
}

// generateTraceID creates a simple trace ID for correlation
func generateTraceID() string {
	return fmt.Sprintf("trace-%d", time.Now().UnixNano())
}

// ============================================================================
// Progress Emitter Factory Support
// ============================================================================

// EmitterMode defines the transport mode for progress reporting
type EmitterMode string

const (
	// ModeStreaming provides real-time progress updates
	ModeStreaming EmitterMode = "streaming"

	// ModeBatched provides batched progress updates for efficiency
	ModeBatched EmitterMode = "batched"

	// ModeTracker uses the legacy progress.Tracker system
	ModeTracker EmitterMode = "tracker"
)

// EmitterConfig configures progress emitter behavior
type EmitterConfig struct {
	Mode           EmitterMode       `json:"mode"`
	BatchSize      int               `json:"batch_size"`
	FlushInterval  time.Duration     `json:"flush_interval"`
	TrackerOptions []progress.Option `json:"-"` // Not serializable
}

// DefaultEmitterConfig returns sensible defaults
func DefaultEmitterConfig() EmitterConfig {
	return EmitterConfig{
		Mode:          ModeStreaming,
		BatchSize:     10,
		FlushInterval: 5 * time.Second,
		TrackerOptions: []progress.Option{
			progress.WithHeartbeat(15 * time.Second),
			progress.WithThrottle(100 * time.Millisecond),
		},
	}
}

// ProgressEmitterFactory creates progress emitters based on transport mode
type ProgressEmitterFactory struct {
	config      EmitterConfig
	sinkFactory *SinkFactory
}

// NewProgressEmitterFactory creates a new progress emitter factory
func NewProgressEmitterFactory(config EmitterConfig, sinkFactory *SinkFactory) *ProgressEmitterFactory {
	return &ProgressEmitterFactory{
		config:      config,
		sinkFactory: sinkFactory,
	}
}

// CreateEmitter creates a ProgressEmitter based on the configured mode
func (f *ProgressEmitterFactory) CreateEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	switch f.config.Mode {
	case ModeStreaming:
		return f.createStreamingEmitter(ctx, req, totalSteps)
	case ModeBatched:
		return f.createBatchedEmitter(ctx, req, totalSteps)
	case ModeTracker:
		return f.createTrackerEmitter(ctx, req, totalSteps)
	default:
		// Fallback to streaming mode
		return f.createStreamingEmitter(ctx, req, totalSteps)
	}
}

func (f *ProgressEmitterFactory) createStreamingEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	// Create underlying tracker emitter
	trackerEmitter := f.createTrackerEmitter(ctx, req, totalSteps)

	// Wrap in streaming emitter for immediate delivery
	return NewStreamingEmitter(trackerEmitter)
}

func (f *ProgressEmitterFactory) createBatchedEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	// Create underlying tracker emitter
	trackerEmitter := f.createTrackerEmitter(ctx, req, totalSteps)

	// Wrap in batched emitter for efficiency
	return NewBatchedEmitter(trackerEmitter, f.config.BatchSize, f.config.FlushInterval)
}

func (f *ProgressEmitterFactory) createTrackerEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	// Create sink for the tracker
	sink := f.sinkFactory.CreateSink(ctx, req)

	// Create tracker emitter with configured options
	return NewTrackerEmitter(ctx, totalSteps, sink, f.config.TrackerOptions...)
}
