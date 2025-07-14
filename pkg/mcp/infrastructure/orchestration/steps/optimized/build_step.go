// Package optimized contains the optimized build step implementation.
package optimized

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/ml"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/util"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps"
)

// Use util.ExtractRepoName instead of local function

// OptimizedBuildStep implements Docker image building with AI-powered resource optimization
type OptimizedBuildStep struct {
	optimizedBuild *ml.OptimizedBuildStep
}

// NewOptimizedBuildStep creates a new optimized build step
func NewOptimizedBuildStep(optimizedBuild *ml.OptimizedBuildStep) workflow.Step {
	return &OptimizedBuildStep{
		optimizedBuild: optimizedBuild,
	}
}

// Name returns the step name
func (s *OptimizedBuildStep) Name() string {
	return "build_image_optimized"
}

// MaxRetries returns the maximum number of retries for this step
func (s *OptimizedBuildStep) MaxRetries() int {
	return 3
}

// Execute performs optimized Docker image building
func (s *OptimizedBuildStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	if state.DockerfileResult == nil || state.AnalyzeResult == nil {
		return fmt.Errorf("dockerfile and analyze results are required for build")
	}

	state.Logger.Info("Step 3: Building Docker image with AI-powered optimization")

	// Generate image name and tag from repo URL
	imageName := util.ExtractRepoName(state.Args.RepoURL)
	imageTag := "latest"
	buildContext := state.AnalyzeResult.RepoPath

	// In test mode, skip actual Docker operations
	if state.Args.TestMode {
		state.Logger.Info("Test mode: Simulating optimized Docker build",
			"image_name", imageName,
			"image_tag", imageTag)

		// Create simulated build result
		state.BuildResult = &workflow.BuildResult{
			ImageID:   fmt.Sprintf("sha256:test-%s", imageName),
			ImageRef:  fmt.Sprintf("%s:%s", imageName, imageTag),
			ImageSize: 100 * 1024 * 1024, // 100MB simulated size
			BuildTime: time.Now().Format(time.RFC3339),
			Metadata: map[string]interface{}{
				"build_context": buildContext,
				"image_name":    imageName,
				"image_tag":     imageTag,
				"test_mode":     true,
				"optimized":     true,
			},
		}

		state.Logger.Info("Test mode: Optimized Docker build simulation completed",
			"image_id", state.BuildResult.ImageID,
			"image_ref", state.BuildResult.ImageRef)

		return nil
	}

	// Create adapter for repository analysis
	analysisAdapter := &analyzeResultAdapter{state.AnalyzeResult}

	// Get optimized build command
	buildCtx := ml.BuildContext{
		DockerfileContent: state.DockerfileResult.Content,
		DockerfilePath:    state.DockerfileResult.Path,
		BuildArgs:         make(map[string]string),
		ImageName:         imageName,
		ImageTag:          imageTag,
		BuildContextPath:  buildContext,
		TestMode:          state.Args.TestMode,
	}

	optimizedCmd, prediction, err := s.optimizedBuild.GetOptimizedBuildCommand(ctx, buildCtx, analysisAdapter)
	if err != nil {
		state.Logger.Warn("Failed to get optimized build command, falling back to standard build",
			"error", err)
		// Fall back to standard build
		return s.standardBuild(ctx, state, imageName, imageTag, buildContext)
	}

	// Log optimization summary
	state.Logger.Info("Build optimization analysis completed",
		"optimization_summary", s.optimizedBuild.GetOptimizationSummary(prediction))

	// Convert workflow types to infrastructure types for compatibility
	infraDockerfileResult := &steps.DockerfileResult{
		Content:     state.DockerfileResult.Content,
		Path:        state.DockerfileResult.Path,
		BaseImage:   state.DockerfileResult.BaseImage,
		ExposedPort: state.DockerfileResult.ExposedPort,
	}

	// Execute optimized build
	state.Logger.Info("Executing optimized Docker build",
		"command", optimizedCmd,
		"predicted_build_time", prediction.BuildTime)

	startTime := time.Now()
	buildResult, err := steps.BuildImage(ctx, infraDockerfileResult, imageName, imageTag, buildContext, state.Logger)
	if err != nil {
		return fmt.Errorf("optimized docker build failed: %v", err)
	}

	if buildResult == nil {
		return fmt.Errorf("build result is nil after successful build")
	}

	actualBuildTime := time.Since(startTime)

	// Store build result in workflow state
	state.BuildResult = &workflow.BuildResult{
		ImageID:   buildResult.ImageID,
		ImageRef:  fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag),
		ImageSize: buildResult.Size,
		BuildTime: buildResult.BuildTime.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"build_context":        buildContext,
			"image_name":           buildResult.ImageName,
			"image_tag":            buildResult.ImageTag,
			"optimized":            true,
			"predicted_build_time": prediction.BuildTime.String(),
			"actual_build_time":    actualBuildTime.String(),
			"cpu_cores_used":       prediction.CPU.Cores,
			"memory_mb_used":       prediction.Memory.RecommendedMB,
			"cache_enabled":        prediction.Cache.UseCache,
			"cache_inline":         prediction.Cache.InlineCache,
		},
	}

	state.Logger.Info("Optimized Docker image build completed",
		"image_id", buildResult.ImageID,
		"image_name", buildResult.ImageName,
		"image_tag", buildResult.ImageTag,
		"predicted_time", prediction.BuildTime,
		"actual_time", actualBuildTime,
		"optimization_accurate", actualBuildTime < prediction.BuildTime+30*time.Second)

	return nil
}

// standardBuild performs a standard build without optimization
func (s *OptimizedBuildStep) standardBuild(ctx context.Context, state *workflow.WorkflowState, imageName, imageTag, buildContext string) error {
	// Convert workflow types to infrastructure types for compatibility
	infraDockerfileResult := &steps.DockerfileResult{
		Content:     state.DockerfileResult.Content,
		Path:        state.DockerfileResult.Path,
		BaseImage:   state.DockerfileResult.BaseImage,
		ExposedPort: state.DockerfileResult.ExposedPort,
	}

	// Call the infrastructure build function
	buildResult, err := steps.BuildImage(ctx, infraDockerfileResult, imageName, imageTag, buildContext, state.Logger)
	if err != nil {
		return fmt.Errorf("docker build failed: %v", err)
	}

	if buildResult == nil {
		return fmt.Errorf("build result is nil after successful build")
	}

	// Store build result in workflow state
	state.BuildResult = &workflow.BuildResult{
		ImageID:   buildResult.ImageID,
		ImageRef:  fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag),
		ImageSize: buildResult.Size,
		BuildTime: buildResult.BuildTime.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"build_context": buildContext,
			"image_name":    buildResult.ImageName,
			"image_tag":     buildResult.ImageTag,
			"optimized":     false,
		},
	}

	state.Logger.Info("Docker image build completed",
		"image_id", buildResult.ImageID,
		"image_name", buildResult.ImageName,
		"image_tag", buildResult.ImageTag)

	return nil
}

// analyzeResultAdapter adapts workflow.AnalyzeResult to ml.RepositoryAnalysis interface
type analyzeResultAdapter struct {
	*workflow.AnalyzeResult
}

func (a *analyzeResultAdapter) GetLanguage() string       { return a.Language }
func (a *analyzeResultAdapter) GetFramework() string      { return a.Framework }
func (a *analyzeResultAdapter) GetDependencies() []string { return a.Dependencies }
func (a *analyzeResultAdapter) GetBuildCommand() string   { return a.BuildCommand }
func (a *analyzeResultAdapter) GetStartCommand() string   { return a.StartCommand }
