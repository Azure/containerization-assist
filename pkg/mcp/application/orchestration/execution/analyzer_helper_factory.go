package execution

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/rs/zerolog"
)

// AnalyzerHelperFactory version that uses the factory pattern to avoid import cycles
type AnalyzerHelperFactory struct {
	analyzer              core.Analyzer
	enhancedBuildAnalyzer interface{} // Avoid importing internal/build
	sessionManager        session.UnifiedSessionManager
	logger                zerolog.Logger
	factory               api.ToolFactory
}

// NewAnalyzerHelperFactory creates a new analyzer helper using the factory pattern
func NewAnalyzerHelperFactory(
	aiAnalyzer core.AIAnalyzer,
	factory api.ToolFactory,
	logger zerolog.Logger,
) *AnalyzerHelperFactory {
	var analyzer core.Analyzer
	if aiAnalyzer != nil && factory != nil {
		if result := factory.CreateAnalyzer(aiAnalyzer); result != nil {
			if coreAnalyzer, ok := result.(core.Analyzer); ok {
				analyzer = coreAnalyzer
			}
		}
	}

	return &AnalyzerHelperFactory{
		analyzer: analyzer,
		factory:  factory,
		logger:   logger,
	}
}

// NewAnalyzerHelperFactoryWithSessionManager creates analyzer helper with session manager
func NewAnalyzerHelperFactoryWithSessionManager(
	aiAnalyzer core.AIAnalyzer,
	sessionManager session.UnifiedSessionManager,
	factory api.ToolFactory,
	logger zerolog.Logger,
) *AnalyzerHelperFactory {
	helper := NewAnalyzerHelperFactory(aiAnalyzer, factory, logger)
	helper.sessionManager = sessionManager
	return helper
}

// GetAnalyzer returns the analyzer
func (h *AnalyzerHelperFactory) GetAnalyzer() core.Analyzer {
	return h.analyzer
}

// GetEnhancedBuildAnalyzer returns the enhanced build analyzer
func (h *AnalyzerHelperFactory) GetEnhancedBuildAnalyzer() interface{} {
	if h.enhancedBuildAnalyzer == nil && h.factory != nil {
		h.enhancedBuildAnalyzer = h.factory.CreateEnhancedBuildAnalyzer()
	}
	return h.enhancedBuildAnalyzer
}

// SetEnhancedBuildAnalyzer sets a custom enhanced build analyzer
func (h *AnalyzerHelperFactory) SetEnhancedBuildAnalyzer(analyzer interface{}) {
	h.enhancedBuildAnalyzer = analyzer
}

// GetSessionManager returns the session manager
func (h *AnalyzerHelperFactory) GetSessionManager() session.UnifiedSessionManager {
	return h.sessionManager
}

// SetupAnalyzeToolAnalyzer creates an analyzer for analyze tools
func (h *AnalyzerHelperFactory) SetupAnalyzeToolAnalyzer(toolName string) core.Analyzer {
	if h.analyzer == nil {
		return nil
	}
	// Return the unified analyzer directly for analyze tools
	return h.analyzer
}

// SetupBuildToolAnalyzer creates an analyzer for build tools
func (h *AnalyzerHelperFactory) SetupBuildToolAnalyzer(toolName string) interface{} {
	return h.GetEnhancedBuildAnalyzer()
}
