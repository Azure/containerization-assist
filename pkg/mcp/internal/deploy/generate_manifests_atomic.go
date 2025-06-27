package deploy

import (
	"context"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// Type aliases for atomic manifest generation to maintain backward compatibility
type AtomicGenerateManifestsArgs = GenerateManifestsArgs
type AtomicGenerateManifestsResult = GenerateManifestsResult

// AtomicGenerateManifestsTool is a simple stub for backward compatibility
type AtomicGenerateManifestsTool struct {
	logger   zerolog.Logger
	baseTool *GenerateManifestsTool
}

// NewAtomicGenerateManifestsTool creates a basic atomic tool for compatibility
func NewAtomicGenerateManifestsTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicGenerateManifestsTool {
	baseTool := NewGenerateManifestsTool(logger, "/tmp/container-kit")
	return &AtomicGenerateManifestsTool{
		logger:   logger.With().Str("tool", "atomic_generate_manifests").Logger(),
		baseTool: baseTool,
	}
}

// GetName returns the tool name
func (t *AtomicGenerateManifestsTool) GetName() string {
	return "atomic_generate_manifests"
}

// Execute delegates to the base tool
func (t *AtomicGenerateManifestsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	return t.baseTool.Execute(ctx, args)
}

// SetAnalyzer is a compatibility method
func (t *AtomicGenerateManifestsTool) SetAnalyzer(analyzer interface{}) {
	// No-op for compatibility
	t.logger.Debug().Msg("SetAnalyzer called on atomic tool (no-op)")
}
