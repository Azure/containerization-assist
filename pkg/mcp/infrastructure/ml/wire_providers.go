// Package ml provides Wire providers for ML components.
package ml

import (
	"log/slog"

	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// ProvideResourcePredictor provides a ResourcePredictor instance
func ProvideResourcePredictor(samplingClient domainsampling.UnifiedSampler, logger *slog.Logger) *ResourcePredictor {
	return NewResourcePredictor(samplingClient, logger)
}

// ProvideBuildOptimizer provides a BuildOptimizer instance
func ProvideBuildOptimizer(predictor *ResourcePredictor, logger *slog.Logger) *BuildOptimizer {
	return NewBuildOptimizer(predictor, logger)
}

// ProvideOptimizedBuildStep provides an OptimizedBuildStep instance
func ProvideOptimizedBuildStep(samplingClient domainsampling.UnifiedSampler, logger *slog.Logger) *OptimizedBuildStep {
	return NewOptimizedBuildStep(samplingClient, logger)
}
