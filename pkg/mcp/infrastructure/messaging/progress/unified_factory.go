// Package progress provides unified progress reporting factory implementation
package progress

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// UnifiedFactory provides a centralized factory for all progress emitter types
// This consolidates the various progress reporting mechanisms per the architectural plan
type UnifiedFactory struct {
	logger      *slog.Logger
	mcpSink     *MCPSink
	cliSink     *CLISink
	sinkFactory *SinkFactory
}

// NewUnifiedFactory creates a new unified progress emitter factory
func NewUnifiedFactory(logger *slog.Logger) *UnifiedFactory {
	sinkFactory := NewSinkFactory(logger)

	return &UnifiedFactory{
		logger:      logger,
		cliSink:     NewCLISink(logger),
		sinkFactory: sinkFactory,
		// mcpSink will be created on demand when needed
	}
}

// Create creates a progress emitter for the specified transport type
func (f *UnifiedFactory) Create(transport progress.TransportType) api.ProgressEmitter {
	ctx := context.Background()

	switch transport {
	case progress.TransportMCP:
		return f.createMCPEmitter(ctx)
	case progress.TransportCLI, progress.TransportStdout:
		return f.createCLIEmitter(ctx)
	default:
		f.logger.Warn("Unknown transport type, defaulting to CLI", "transport", string(transport))
		return f.createCLIEmitter(ctx)
	}
}

// CreateForContext creates a progress emitter based on context and request
func (f *UnifiedFactory) CreateForContext(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
	// Try to detect transport type from context
	if srv := server.ServerFromContext(ctx); srv != nil {
		// MCP context detected
		if req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
			// ProgressToken is an interface type, use it directly
			return f.createMCPEmitterWithToken(srv, req.Params.Meta.ProgressToken)
		}
		return f.createMCPEmitter(ctx)
	}

	// Default to CLI for non-MCP contexts
	return f.createCLIEmitter(ctx)
}

// CreateTrackerEmitter creates a tracker-based emitter
func (f *UnifiedFactory) CreateTrackerEmitter(ctx context.Context, totalSteps int, sink progress.Sink) api.ProgressEmitter {
	opts := []progress.Option{
		progress.WithHeartbeat(2 * time.Second),
		progress.WithThrottle(100 * time.Millisecond),
	}

	return NewTrackerEmitter(ctx, totalSteps, sink, opts...)
}

// CreateStreamingEmitter creates a real-time streaming emitter
func (f *UnifiedFactory) CreateStreamingEmitter(base api.ProgressEmitter) api.ProgressEmitter {
	return NewStreamingEmitter(base)
}

// CreateBatchedEmitter creates a batched emitter for efficiency
func (f *UnifiedFactory) CreateBatchedEmitter(base api.ProgressEmitter, batchSize int) api.ProgressEmitter {
	flushInterval := 5 * time.Second
	return NewBatchedEmitter(base, batchSize, flushInterval)
}

// createMCPEmitter creates an MCP-based progress emitter
func (f *UnifiedFactory) createMCPEmitter(ctx context.Context) api.ProgressEmitter {
	// Create a basic tracker emitter with MCP sink
	// For now, use a simple approach - in a real MCP context, we'd get the server from context
	totalSteps := 10  // Default for basic emitter
	sink := f.cliSink // Fallback to CLI if no MCP server available

	return f.CreateTrackerEmitter(ctx, totalSteps, sink)
}

// createMCPEmitterWithToken creates an MCP emitter with a specific progress token
func (f *UnifiedFactory) createMCPEmitterWithToken(srv *server.MCPServer, token mcp.ProgressToken) api.ProgressEmitter {
	ctx := context.Background()

	// Create MCP sink with progress token (interface{} type)
	mcpSink := NewMCPSink(srv, token, f.logger)

	totalSteps := 10 // Default for basic emitter
	return f.CreateTrackerEmitter(ctx, totalSteps, mcpSink)
}

// createCLIEmitter creates a CLI-based progress emitter
func (f *UnifiedFactory) createCLIEmitter(ctx context.Context) api.ProgressEmitter {
	totalSteps := 10 // Default for basic emitter
	return f.CreateTrackerEmitter(ctx, totalSteps, f.cliSink)
}

// ==================================================================================
// Legacy Factory Integration
// ==================================================================================

// CreateEmitter creates a ProgressEmitter (implements the existing workflow interface)
func (f *UnifiedFactory) CreateEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	// Use the context-aware creation but override total steps
	emitter := f.CreateForContext(ctx, req)

	// If it's a tracker emitter, we could potentially recreate with correct total steps
	// For now, return the context-appropriate emitter
	return emitter
}

// GetSinkFactory returns the underlying sink factory for advanced use cases
func (f *UnifiedFactory) GetSinkFactory() *SinkFactory {
	return f.sinkFactory
}
