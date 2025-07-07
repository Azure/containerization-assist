package analyze

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
)

// CreateAnalyzer creates the appropriate Analyzer implementation based on available components
// This factory function handles all possible combinations of repository and AI analyzers
func CreateAnalyzer(
	repoAnalyzer core.RepositoryAnalyzer,
	aiAnalyzer core.AIAnalyzer,
	logger *slog.Logger,
) core.Analyzer {
	// Both available - create unified analyzer
	if repoAnalyzer != nil && aiAnalyzer != nil {
		return NewUnifiedAnalyzer(repoAnalyzer, aiAnalyzer, logger)
	}

	// Only repository analyzer available
	if repoAnalyzer != nil {
		return NewPartialRepositoryAnalyzer(repoAnalyzer)
	}

	// Only AI analyzer available
	if aiAnalyzer != nil {
		return NewPartialAIAnalyzer(aiAnalyzer)
	}

	// Neither available
	return nil
}

// CreateAnalyzerFromInterface creates an Analyzer from any analyzer interface
// This is useful during the migration period when you might receive either type
func CreateAnalyzerFromInterface(analyzer interface{}, logger *slog.Logger) core.Analyzer {
	switch a := analyzer.(type) {
	case core.Analyzer:
		// Already unified
		return a
	case core.AIAnalyzer:
		// Wrap in partial implementation
		return NewPartialAIAnalyzer(a)
	case core.RepositoryAnalyzer:
		// Wrap in partial implementation
		return NewPartialRepositoryAnalyzer(a)
	default:
		logger.Warn("Unknown analyzer type, returning nil",
			"type", analyzer)
		return nil
	}
}

// MigrateToUnified helps migrate existing code that uses separate analyzers
// This is a helper function for the migration period
func MigrateToUnified(
	repoAnalyzer core.RepositoryAnalyzer,
	aiAnalyzer core.AIAnalyzer,
	logger *slog.Logger,
) (core.Analyzer, core.RepositoryAnalyzer, core.AIAnalyzer) {
	// Create unified analyzer
	unified := CreateAnalyzer(repoAnalyzer, aiAnalyzer, logger)

	if unified == nil {
		return nil, nil, nil
	}

	// Return unified analyzer directly - adapters are no longer needed
	return unified, nil, nil
}

// ValidateAnalyzer checks if an analyzer has specific capabilities
// This helps with graceful degradation when features aren't available
func ValidateAnalyzer(analyzer core.Analyzer) AnalyzerCapabilities {
	capabilities := AnalyzerCapabilities{}

	if analyzer == nil {
		return capabilities
	}

	// Check repository capabilities by trying a non-destructive operation
	// We can't directly test without causing side effects, so we use type introspection
	switch analyzer.(type) {
	case *UnifiedAnalyzer:
		capabilities.HasRepository = true
		capabilities.HasAI = true
	case *PartialRepositoryAnalyzer:
		capabilities.HasRepository = true
		capabilities.HasAI = false
	case *PartialAIAnalyzer:
		capabilities.HasRepository = false
		capabilities.HasAI = true
	default:
		// Unknown implementation - assume it has everything
		capabilities.HasRepository = true
		capabilities.HasAI = true
	}

	return capabilities
}

// AnalyzerCapabilities describes what features an analyzer supports
type AnalyzerCapabilities struct {
	HasRepository bool `json:"has_repository"`
	HasAI         bool `json:"has_ai"`
}

// CanAnalyzeRepository returns true if the analyzer can perform repository analysis
func (c AnalyzerCapabilities) CanAnalyzeRepository() bool {
	return c.HasRepository
}

// CanAnalyzeWithAI returns true if the analyzer can perform AI analysis
func (c AnalyzerCapabilities) CanAnalyzeWithAI() bool {
	return c.HasAI
}

// IsFullyFunctional returns true if the analyzer supports both repository and AI analysis
func (c AnalyzerCapabilities) IsFullyFunctional() bool {
	return c.HasRepository && c.HasAI
}
