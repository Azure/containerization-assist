package deploy

import (
	"context"

	// mcp import removed - using mcptypes

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/rs/zerolog"
)

// Type aliases for atomic manifest generation to maintain backward compatibility
type AtomicGenerateManifestsArgs = GenerateManifestsArgs
type AtomicGenerateManifestsResult = GenerateManifestsResult

// AtomicGenerateManifestsTool is a simple stub for backward compatibility
type AtomicGenerateManifestsTool struct {
	logger         zerolog.Logger
	baseTool       *GenerateManifestsTool
	sessionManager core.ToolSessionManager
}

// NewAtomicGenerateManifestsTool creates a basic atomic tool for compatibility
func NewAtomicGenerateManifestsTool(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicGenerateManifestsTool {
	baseTool := NewGenerateManifestsTool(logger, "/tmp/container-kit")
	return &AtomicGenerateManifestsTool{
		logger:         logger.With().Str("tool", "atomic_generate_manifests").Logger(),
		baseTool:       baseTool,
		sessionManager: sessionManager,
	}
}

// GetName returns the tool name
func (t *AtomicGenerateManifestsTool) GetName() string {
	return "atomic_generate_manifests"
}

// Execute delegates to the base tool with session workspace handling
func (t *AtomicGenerateManifestsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Extract session ID from args to get workspace
	var sessionID string
	switch v := args.(type) {
	case GenerateManifestsArgs:
		sessionID = v.SessionID
	case map[string]interface{}:
		if sid, ok := v["session_id"].(string); ok {
			sessionID = sid
		}
	}

	// Get workspace directory from session if available
	if sessionID != "" && t.sessionManager != nil {
		sessionInterface, err := t.sessionManager.GetSession(sessionID)
		if err == nil {
			// Extract workspace directory from session
			if sessionState, ok := sessionInterface.(*session.SessionState); ok && sessionState.WorkspaceDir != "" {
				// Create a new base tool with the correct workspace
				t.baseTool = NewGenerateManifestsTool(t.logger, sessionState.WorkspaceDir)
			}
		}
	}

	return t.baseTool.Execute(ctx, args)
}

// GetMetadata delegates to the base tool
func (t *AtomicGenerateManifestsTool) GetMetadata() core.ToolMetadata {
	return t.baseTool.GetMetadata()
}

// Validate delegates to the base tool
func (t *AtomicGenerateManifestsTool) Validate(ctx context.Context, args interface{}) error {
	return t.baseTool.Validate(ctx, args)
}

// SetAnalyzer is a compatibility method
func (t *AtomicGenerateManifestsTool) SetAnalyzer(analyzer interface{}) {
	// No-op for compatibility
	t.logger.Debug().Msg("SetAnalyzer called on atomic tool (no-op)")
}
