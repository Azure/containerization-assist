package execution

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/rs/zerolog"
)

// AnalyzerHelperV2 provides analyzer initialization without import cycles
type AnalyzerHelperV2 struct {
	analyzer              core.Analyzer
	enhancedBuildAnalyzer interface{} // Avoid importing internal/build
	sessionManager        session.UnifiedSessionManager
	logger                zerolog.Logger
	factory               api.ToolFactory
}

// NewAnalyzerHelperV2 creates a new analyzer helper using the factory pattern
func NewAnalyzerHelperV2(
	aiAnalyzer core.AIAnalyzer,
	factory api.ToolFactory,
	logger zerolog.Logger,
) *AnalyzerHelperV2 {
	var analyzer core.Analyzer
	if aiAnalyzer != nil && factory != nil {
		if result := factory.CreateAnalyzer(aiAnalyzer); result != nil {
			if coreAnalyzer, ok := result.(core.Analyzer); ok {
				analyzer = coreAnalyzer
			}
		}
	}

	return &AnalyzerHelperV2{
		analyzer: analyzer,
		factory:  factory,
		logger:   logger,
	}
}

// NewAnalyzerHelperV2WithSessionManager creates analyzer helper with session manager
func NewAnalyzerHelperV2WithSessionManager(
	aiAnalyzer core.AIAnalyzer,
	sessionManager session.UnifiedSessionManager,
	factory api.ToolFactory,
	logger zerolog.Logger,
) *AnalyzerHelperV2 {
	helper := NewAnalyzerHelperV2(aiAnalyzer, factory, logger)
	helper.sessionManager = sessionManager
	return helper
}

// GetAnalyzer returns the analyzer
func (h *AnalyzerHelperV2) GetAnalyzer() core.Analyzer {
	return h.analyzer
}

// GetEnhancedBuildAnalyzer returns the enhanced build analyzer
func (h *AnalyzerHelperV2) GetEnhancedBuildAnalyzer() interface{} {
	if h.enhancedBuildAnalyzer == nil && h.factory != nil {
		h.enhancedBuildAnalyzer = h.factory.CreateEnhancedBuildAnalyzer()
	}
	return h.enhancedBuildAnalyzer
}

// SetEnhancedBuildAnalyzer sets a custom enhanced build analyzer
func (h *AnalyzerHelperV2) SetEnhancedBuildAnalyzer(analyzer interface{}) {
	h.enhancedBuildAnalyzer = analyzer
}

// GetSessionManager returns the session manager
func (h *AnalyzerHelperV2) GetSessionManager() session.UnifiedSessionManager {
	return h.sessionManager
}
