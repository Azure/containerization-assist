package scan

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/rs/zerolog"
)

// ToolAnalyzer interface defines the contract for scan tool analyzers
// Deprecated: Use common.FailureAnalyzer for unified failure analysis
type ToolAnalyzer interface {
	AnalyzeScanFailure(imageRef, sessionID string) error
	AnalyzeSecretsFailure(path, sessionID string) error
}

// DefaultToolAnalyzer provides default implementation for scan tool analysis
type DefaultToolAnalyzer struct {
	*common.DefaultFailureAnalyzer
	toolName string
	logger   zerolog.Logger
}

// NewDefaultToolAnalyzer creates a new default scan tool analyzer
func NewDefaultToolAnalyzer(toolName string) *DefaultToolAnalyzer {
	logger := zerolog.Nop()
	return &DefaultToolAnalyzer{
		DefaultFailureAnalyzer: common.NewDefaultFailureAnalyzer(toolName, "scan", logger),
		toolName:               toolName,
		logger:                 logger,
	}
}

// SetLogger sets the logger for the analyzer
func (a *DefaultToolAnalyzer) SetLogger(logger zerolog.Logger) {
	a.logger = logger.With().Str("component", "scan_analyzer").Str("tool", a.toolName).Logger()
}

// AnalyzeScanFailure analyzes security scan failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzeScanFailure(imageRef, sessionID string) error {
	params := map[string]interface{}{
		"image_ref": imageRef,
	}
	return a.AnalyzeFailure(context.Background(), "scan", sessionID, params)
}

// AnalyzeSecretsFailure analyzes secrets scan failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzeSecretsFailure(path, sessionID string) error {
	params := map[string]interface{}{
		"path": path,
	}
	return a.AnalyzeFailure(context.Background(), "secrets", sessionID, params)
}
