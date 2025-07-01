package analyze

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/rs/zerolog"
)

// ToolAnalyzer provides analysis capabilities for atomic analysis tools
// Deprecated: Use common.FailureAnalyzer for unified failure analysis
type ToolAnalyzer interface {
	AnalyzeValidationFailure(dockerfilePath, sessionID string) error
}

// DefaultToolAnalyzer provides a default implementation using the unified analyzer
type DefaultToolAnalyzer struct {
	*common.DefaultFailureAnalyzer
}

// NewDefaultToolAnalyzer creates a new default tool analyzer
func NewDefaultToolAnalyzer(toolName string) *DefaultToolAnalyzer {
	return &DefaultToolAnalyzer{
		DefaultFailureAnalyzer: common.NewDefaultFailureAnalyzer(toolName, "analyze", zerolog.Nop()),
	}
}

// AnalyzeValidationFailure analyzes validation failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzeValidationFailure(dockerfilePath, sessionID string) error {
	params := map[string]interface{}{
		"dockerfile_path": dockerfilePath,
	}
	return a.AnalyzeFailure(context.Background(), "validation", sessionID, params)
}
