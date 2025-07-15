// Package progress provides domain interfaces for progress reporting
package progress

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/mcp"
)

// TransportType represents the type of transport for progress reporting
type TransportType string

const (
	// TransportMCP represents MCP protocol-based progress reporting
	TransportMCP TransportType = "mcp"

	// TransportCLI represents command-line interface progress reporting
	TransportCLI TransportType = "cli"

	// TransportStdout represents standard output progress reporting
	TransportStdout TransportType = "stdout"
)

// EmitterFactory creates progress emitters for different transport types
// This provides a unified interface for creating appropriate progress reporters
// based on the transport context.
type EmitterFactory interface {
	// Create creates a progress emitter for the specified transport type
	Create(transport TransportType) api.ProgressEmitter

	// CreateForContext creates a progress emitter based on context and request
	CreateForContext(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter
}

// UnifiedEmitterFactory provides a centralized factory for all progress emitter types
// This consolidates the various progress reporting mechanisms into a single interface
type UnifiedEmitterFactory interface {
	EmitterFactory

	// CreateTrackerEmitter creates a tracker-based emitter
	CreateTrackerEmitter(ctx context.Context, totalSteps int, sink Sink) api.ProgressEmitter

	// CreateStreamingEmitter creates a real-time streaming emitter
	CreateStreamingEmitter(base api.ProgressEmitter) api.ProgressEmitter

	// CreateBatchedEmitter creates a batched emitter for efficiency
	CreateBatchedEmitter(base api.ProgressEmitter, batchSize int) api.ProgressEmitter
}
