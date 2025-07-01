package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/rs/zerolog"
)

// ToolAnalyzer provides analysis capabilities for atomic tools
// Deprecated: Use common.FailureAnalyzer for unified failure analysis
type ToolAnalyzer interface {
	AnalyzeBuildFailure(sessionID, imageName string) error
	AnalyzePushFailure(imageRef, sessionID string) error
	AnalyzePullFailure(imageRef, sessionID string) error
	AnalyzeTagFailure(sourceImage, targetImage, sessionID string) error
}

// DefaultToolAnalyzer provides a default implementation using the unified analyzer
type DefaultToolAnalyzer struct {
	*common.DefaultFailureAnalyzer
}

// NewDefaultToolAnalyzer creates a new default tool analyzer
func NewDefaultToolAnalyzer(toolName string) *DefaultToolAnalyzer {
	return &DefaultToolAnalyzer{
		DefaultFailureAnalyzer: common.NewDefaultFailureAnalyzer(toolName, "build", zerolog.Nop()),
	}
}

// AnalyzeBuildFailure analyzes build failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzeBuildFailure(sessionID, imageName string) error {
	params := map[string]interface{}{
		"image_name": imageName,
	}
	return a.AnalyzeFailure(context.Background(), "build", sessionID, params)
}

// AnalyzePushFailure analyzes push failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzePushFailure(imageRef, sessionID string) error {
	params := map[string]interface{}{
		"image_ref": imageRef,
	}
	return a.AnalyzeFailure(context.Background(), "push", sessionID, params)
}

// AnalyzePullFailure analyzes pull failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzePullFailure(imageRef, sessionID string) error {
	params := map[string]interface{}{
		"image_ref": imageRef,
	}
	return a.AnalyzeFailure(context.Background(), "pull", sessionID, params)
}

// AnalyzeTagFailure analyzes tag failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzeTagFailure(sourceImage, targetImage, sessionID string) error {
	params := map[string]interface{}{
		"source_image": sourceImage,
		"target_image": targetImage,
	}
	return a.AnalyzeFailure(context.Background(), "tag", sessionID, params)
}
