// Package ml provides integration between optimized build and workflow.
package ml

import (
	"context"
	"fmt"
	"log/slog"

	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// BuildContext provides context for a build operation
type BuildContext struct {
	DockerfileContent string
	DockerfilePath    string
	BuildArgs         map[string]string
	ImageName         string
	ImageTag          string
	BuildContextPath  string
	TestMode          bool
}

// BuildOutput represents the result of a build operation
type BuildOutput struct {
	ImageName string
	ImageTag  string
	ImageID   string
	BuildTime string
}

// OptimizedBuildStep integrates AI-powered resource optimization into the workflow build step
type OptimizedBuildStep struct {
	samplingClient domainsampling.UnifiedSampler
	logger         *slog.Logger
}

// NewOptimizedBuildStep creates a new optimized build step
func NewOptimizedBuildStep(samplingClient domainsampling.UnifiedSampler, logger *slog.Logger) *OptimizedBuildStep {
	return &OptimizedBuildStep{
		samplingClient: samplingClient,
		logger:         logger.With("component", "optimized_build_step"),
	}
}

// PredictBuildResources predicts optimal resources for a build
func (s *OptimizedBuildStep) PredictBuildResources(
	ctx context.Context,
	analysis RepositoryAnalysis,
) (*ResourcePrediction, error) {

	// Create predictor
	predictor := NewResourcePredictor(s.samplingClient, s.logger)

	// Get resource predictions
	prediction, err := predictor.PredictResources(ctx, analysis)
	if err != nil {
		s.logger.Error("Failed to predict resources", "error", err)
		return nil, err
	}

	s.logger.Info("Build resources predicted",
		"cpu_cores", prediction.CPU.Cores,
		"memory_mb", prediction.Memory.RecommendedMB,
		"confidence", prediction.Confidence)

	return prediction, nil
}

// GetOptimizedBuildCommand creates an optimized build command
func (s *OptimizedBuildStep) GetOptimizedBuildCommand(
	ctx context.Context,
	buildCtx BuildContext,
	analysis RepositoryAnalysis,
) (string, *ResourcePrediction, error) {

	// Get predictions
	prediction, err := s.PredictBuildResources(ctx, analysis)
	if err != nil {
		return "", nil, err
	}

	// Create optimizer
	predictor := NewResourcePredictor(s.samplingClient, s.logger)
	optimizer := NewBuildOptimizer(predictor, s.logger)

	// Generate optimized command
	tags := []string{fmt.Sprintf("%s:%s", buildCtx.ImageName, buildCtx.ImageTag)}

	optimizedCmd, _, err := optimizer.OptimizeBuildCommand(
		ctx,
		"docker buildx build",
		analysis,
		buildCtx.DockerfilePath,
		buildCtx.BuildContextPath,
		tags,
	)

	return optimizedCmd, prediction, err
}

// GetOptimizationSummary returns a human-readable summary of optimizations
func (s *OptimizedBuildStep) GetOptimizationSummary(prediction *ResourcePrediction) string {
	if prediction == nil {
		return "No optimization predictions available"
	}

	summary := fmt.Sprintf(`Build Optimization Applied:
- CPU: %d cores (parallelism: %d)
- Memory: %d MB
- Estimated build time: %v
- Cache mounts: %d
- Confidence: %.0f%%`,
		prediction.CPU.Cores,
		prediction.CPU.ParallelismLevel,
		prediction.Memory.RecommendedMB,
		prediction.BuildTime,
		len(prediction.Cache.MountCaches),
		prediction.Confidence*100)

	if len(prediction.Recommendations) > 0 {
		summary += "\n\nRecommendations:"
		for _, rec := range prediction.Recommendations {
			summary += fmt.Sprintf("\n- %s", rec)
		}
	}

	return summary
}
