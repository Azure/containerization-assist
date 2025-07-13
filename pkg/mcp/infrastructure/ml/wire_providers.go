// Package ml provides Wire providers for ML components.
package ml

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// ProvideResourcePredictor provides a ResourcePredictor instance
func ProvideResourcePredictor(samplingClient *sampling.Client, logger *slog.Logger) *ResourcePredictor {
	return NewResourcePredictor(samplingClient, logger)
}

// ProvideBuildOptimizer provides a BuildOptimizer instance
func ProvideBuildOptimizer(predictor *ResourcePredictor, logger *slog.Logger) *BuildOptimizer {
	return NewBuildOptimizer(predictor, logger)
}

// ProvideOptimizedBuildStep provides an OptimizedBuildStep instance
func ProvideOptimizedBuildStep(samplingClient *sampling.Client, logger *slog.Logger) *OptimizedBuildStep {
	return NewOptimizedBuildStep(samplingClient, logger)
}
