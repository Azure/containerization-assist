// Package steps provides optimized Docker build with AI-powered resource prediction.
package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/ml"
)

// OptimizedBuildResult extends BuildResult with optimization metadata
type OptimizedBuildResult struct {
	*BuildResult
	ResourcePrediction  *ml.ResourcePrediction `json:"resource_prediction,omitempty"`
	ActualResources     *ml.ResourceUsage      `json:"actual_resources,omitempty"`
	OptimizationApplied bool                   `json:"optimization_applied"`
}

// BuildImageOptimized builds a Docker image with AI-powered resource optimization
func BuildImageOptimized(
	ctx context.Context,
	dockerfileResult *DockerfileResult,
	analyzeResult *AnalyzeResult,
	imageName, imageTag, buildContext string,
	samplingClient sampling.UnifiedSampler,
	logger *slog.Logger,
) (*OptimizedBuildResult, error) {

	logger.Info("Starting optimized Docker image build",
		"image_name", imageName,
		"image_tag", imageTag,
		"build_context", buildContext,
		"language", analyzeResult.Language)

	if dockerfileResult == nil {
		return nil, fmt.Errorf("dockerfile result is required")
	}

	if analyzeResult == nil {
		// Fall back to standard build if no analysis available
		logger.Warn("No analysis result available, using standard build")
		result, err := BuildImage(ctx, dockerfileResult, imageName, imageTag, buildContext, logger)
		if err != nil {
			return nil, err
		}
		return &OptimizedBuildResult{
			BuildResult:         result,
			OptimizationApplied: false,
		}, nil
	}

	// Initialize resource predictor
	predictor := ml.NewResourcePredictor(samplingClient, logger)
	optimizer := ml.NewBuildOptimizer(predictor, logger)

	// Get resource predictions
	buildProfile := createBuildProfile(analyzeResult, dockerfileResult)

	// startTime := time.Now() // Reserved for future use

	// Create adapter for analyze result
	analysisAdapter := &analyzeResultAdapter{analyzeResult}

	// Predict optimal resources
	prediction, err := predictor.PredictResources(ctx, analysisAdapter)
	if err != nil {
		logger.Error("Resource prediction failed, falling back to standard build", "error", err)
		// Fall back to standard build
		result, err := BuildImage(ctx, dockerfileResult, imageName, imageTag, buildContext, logger)
		if err != nil {
			return nil, err
		}
		return &OptimizedBuildResult{
			BuildResult:         result,
			OptimizationApplied: false,
		}, nil
	}

	// Log optimization summary
	logger.Info(optimizer.GetOptimizationSummary(prediction))

	// Check if buildx is available
	if !isBuildxAvailable(ctx) {
		logger.Warn("Docker buildx not available, using standard build with resource limits")
		return buildWithResourceLimits(ctx, dockerfileResult, imageName, imageTag, buildContext, prediction, logger)
	}

	// Build with full optimization using buildx
	return buildWithBuildx(ctx, dockerfileResult, imageName, imageTag, buildContext, prediction, optimizer, buildProfile, logger)
}

// createBuildProfile creates a build profile from analysis results
func createBuildProfile(analysis *AnalyzeResult, dockerfile *DockerfileResult) ml.BuildProfile {
	// Extract dependencies from analysis map
	depCount := 0
	if deps, ok := analysis.Analysis["dependencies"].([]interface{}); ok {
		depCount = len(deps)
	}

	// Extract build command from analysis map
	buildCmd := ""
	if cmd, ok := analysis.Analysis["build_command"].(string); ok {
		buildCmd = cmd
	}

	profile := ml.BuildProfile{
		Language:     analysis.Language,
		Framework:    analysis.Framework,
		Dependencies: depCount,
		BuildSystem:  detectBuildSystem(buildCmd),
		// Estimate from dockerfile
		BuildSteps: countDockerfileSteps(dockerfile.Content),
		MultiStage: strings.Contains(dockerfile.Content, "FROM") && strings.Count(dockerfile.Content, "FROM") > 1,
		TestSuite:  hasTestCommands(dockerfile.Content),
	}

	// Rough size estimates (would be enhanced with actual file analysis)
	profile.CodeSizeMB = 50.0  // Default estimate
	profile.AssetSizeMB = 10.0 // Default estimate

	return profile
}

// isBuildxAvailable checks if docker buildx is available
func isBuildxAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "buildx", "version")
	err := cmd.Run()
	return err == nil
}

// buildWithResourceLimits builds with standard docker but applies resource limits
func buildWithResourceLimits(
	ctx context.Context,
	dockerfileResult *DockerfileResult,
	imageName, imageTag, buildContext string,
	prediction *ml.ResourcePrediction,
	logger *slog.Logger,
) (*OptimizedBuildResult, error) {

	// Write optimized Dockerfile with cache mount comments
	optimizedContent := addCacheOptimizations(dockerfileResult.Content, prediction)

	// Create temporary optimized Dockerfile
	tmpfile, err := os.CreateTemp("", "Dockerfile.optimized.*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dockerfile: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(optimizedContent); err != nil {
		return nil, fmt.Errorf("failed to write optimized dockerfile: %w", err)
	}
	tmpfile.Close()

	// Update dockerfile result with optimized content
	optimizedDockerfile := &DockerfileResult{
		Content:   optimizedContent,
		Path:      tmpfile.Name(),
		BuildArgs: dockerfileResult.BuildArgs,
	}

	// Add resource limit build args
	if optimizedDockerfile.BuildArgs == nil {
		optimizedDockerfile.BuildArgs = make(map[string]string)
	}
	optimizedDockerfile.BuildArgs["GOMAXPROCS"] = fmt.Sprintf("%d", prediction.CPU.ParallelismLevel)

	// Build with standard docker
	result, err := BuildImage(ctx, optimizedDockerfile, imageName, imageTag, buildContext, logger)
	if err != nil {
		return nil, err
	}

	return &OptimizedBuildResult{
		BuildResult:         result,
		ResourcePrediction:  prediction,
		OptimizationApplied: true,
	}, nil
}

// buildWithBuildx builds using docker buildx with full optimization
func buildWithBuildx(
	ctx context.Context,
	dockerfileResult *DockerfileResult,
	imageName, imageTag, buildContext string,
	prediction *ml.ResourcePrediction,
	optimizer *ml.BuildOptimizer,
	profile ml.BuildProfile,
	logger *slog.Logger,
) (*OptimizedBuildResult, error) {

	// Generate optimized build command
	baseCmd := "docker buildx build"
	tags := []string{fmt.Sprintf("%s:%s", imageName, imageTag)}

	// Create simple analysis for optimizer
	simpleAnalysis := &ml.SimpleAnalysis{
		Language:     profile.Language,
		Framework:    profile.Framework,
		Dependencies: make([]string, profile.Dependencies), // Placeholder
		BuildCommand: profile.BuildSystem,
		StartCommand: "",
	}

	optimizedCmd, _, err := optimizer.OptimizeBuildCommand(
		ctx,
		baseCmd,
		simpleAnalysis,
		dockerfileResult.Path,
		buildContext,
		tags,
	)
	if err != nil {
		logger.Error("Failed to generate optimized command", "error", err)
		// Fall back to standard build
		result, err := BuildImage(ctx, dockerfileResult, imageName, imageTag, buildContext, logger)
		if err != nil {
			return nil, err
		}
		return &OptimizedBuildResult{
			BuildResult:         result,
			OptimizationApplied: false,
		}, nil
	}

	// Write Dockerfile to temporary file if needed
	tmpfile, err := os.CreateTemp("", "Dockerfile.*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dockerfile: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(dockerfileResult.Content); err != nil {
		return nil, fmt.Errorf("failed to write dockerfile: %w", err)
	}
	tmpfile.Close()

	// Update command with actual Dockerfile path
	optimizedCmd = strings.Replace(optimizedCmd, dockerfileResult.Path, tmpfile.Name(), 1)

	logger.Info("Executing optimized build command", "command", optimizedCmd)

	// Execute optimized build
	cmdParts := strings.Fields(optimizedCmd)
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	cmd.Dir = buildContext

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Optimized build failed",
			"error", err,
			"output", string(output))
		return nil, fmt.Errorf("optimized build failed: %w\nOutput: %s", err, output)
	}

	// Extract image ID from output
	imageID := extractImageID(string(output))

	// Monitor and record performance
	buildID := fmt.Sprintf("build-%d", time.Now().Unix())
	buildRecord := optimizer.MonitorBuildPerformance(ctx, buildID, profile, time.Now())

	result := &OptimizedBuildResult{
		BuildResult: &BuildResult{
			ImageName: imageName,
			ImageTag:  imageTag,
			ImageID:   imageID,
			BuildTime: time.Now(),
		},
		ResourcePrediction:  prediction,
		ActualResources:     &buildRecord.Resources,
		OptimizationApplied: true,
	}

	logger.Info("Optimized build completed successfully",
		"image_id", imageID,
		"duration", buildRecord.Duration,
		"cache_hit_rate", buildRecord.CacheHitRate)

	return result, nil
}

// addCacheOptimizations adds cache mount optimizations to Dockerfile
func addCacheOptimizations(dockerfileContent string, prediction *ml.ResourcePrediction) string {
	if len(prediction.Cache.MountCaches) == 0 {
		return dockerfileContent
	}

	var optimized strings.Builder
	optimized.WriteString("# syntax=docker/dockerfile:1\n")
	optimized.WriteString("# AI-Optimized Dockerfile with cache mounts\n\n")

	lines := strings.Split(dockerfileContent, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Add cache mounts to package manager commands
		if shouldAddCacheMount(trimmed, prediction) {
			optimized.WriteString(addCacheMountToCommand(line, prediction))
		} else {
			optimized.WriteString(line)
		}
		optimized.WriteString("\n")
	}

	return optimized.String()
}

// shouldAddCacheMount checks if a command should have cache mounts added
func shouldAddCacheMount(line string, prediction *ml.ResourcePrediction) bool {
	cacheableCommands := []string{
		"npm install", "npm ci", "yarn install",
		"pip install", "poetry install",
		"go mod download", "go build",
		"cargo build", "cargo fetch",
		"mvn", "gradle",
		"apt-get", "apk add",
	}

	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "run ") {
		return false
	}

	for _, cmd := range cacheableCommands {
		if strings.Contains(lower, cmd) {
			return true
		}
	}
	return false
}

// addCacheMountToCommand adds appropriate cache mount to a RUN command
func addCacheMountToCommand(line string, prediction *ml.ResourcePrediction) string {
	// Find the appropriate cache mount
	var mount *ml.CacheMount
	lower := strings.ToLower(line)

	for _, m := range prediction.Cache.MountCaches {
		if strings.Contains(lower, "npm") && strings.Contains(m.Target, "npm") {
			mount = &m
			break
		} else if strings.Contains(lower, "pip") && strings.Contains(m.Target, "pip") {
			mount = &m
			break
		} else if strings.Contains(lower, "go") && strings.Contains(m.Target, "go") {
			mount = &m
			break
		}
		// Add more matching logic as needed
	}

	if mount == nil {
		return line
	}

	// Insert mount after RUN
	parts := strings.SplitN(line, "RUN", 2)
	if len(parts) != 2 {
		return line
	}

	mountStr := fmt.Sprintf("--mount=type=%s,target=%s", mount.Type, mount.Target)
	if mount.ID != "" {
		mountStr += fmt.Sprintf(",id=%s", mount.ID)
	}
	if mount.Sharing != "" {
		mountStr += fmt.Sprintf(",sharing=%s", mount.Sharing)
	}

	return fmt.Sprintf("%sRUN %s \\\n    %s", parts[0], mountStr, strings.TrimSpace(parts[1]))
}

// countDockerfileSteps counts the number of steps in a Dockerfile
func countDockerfileSteps(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.ToUpper(line))
		if strings.HasPrefix(trimmed, "FROM") ||
			strings.HasPrefix(trimmed, "RUN") ||
			strings.HasPrefix(trimmed, "COPY") ||
			strings.HasPrefix(trimmed, "ADD") {
			count++
		}
	}
	return count
}

// detectBuildSystem detects the build system from the build command
func detectBuildSystem(buildCommand string) string {
	if buildCommand == "" {
		return "unknown"
	}

	// Common build systems
	buildSystems := map[string][]string{
		"npm":    {"npm run", "npm build"},
		"yarn":   {"yarn build", "yarn run"},
		"pnpm":   {"pnpm build", "pnpm run"},
		"maven":  {"mvn ", "maven "},
		"gradle": {"gradle ", "./gradlew"},
		"make":   {"make "},
		"cargo":  {"cargo build"},
		"go":     {"go build"},
		"pip":    {"pip install"},
		"poetry": {"poetry build", "poetry install"},
		"dotnet": {"dotnet build"},
	}

	lowerCmd := strings.ToLower(buildCommand)
	for system, patterns := range buildSystems {
		for _, pattern := range patterns {
			if strings.Contains(lowerCmd, pattern) {
				return system
			}
		}
	}

	return "custom"
}

// hasTestCommands checks if Dockerfile contains test commands
func hasTestCommands(content string) bool {
	testIndicators := []string{
		"test", "pytest", "jest", "mocha", "junit", "rspec", "phpunit",
	}
	lower := strings.ToLower(content)
	for _, indicator := range testIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

// extractImageID extracts the image ID from build output
func extractImageID(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "writing image sha256:") {
			parts := strings.Split(line, "sha256:")
			if len(parts) >= 2 {
				id := strings.TrimSpace(parts[1])
				if spaceIdx := strings.Index(id, " "); spaceIdx > 0 {
					id = id[:spaceIdx]
				}
				return "sha256:" + id
			}
		}
	}
	// Fallback: look for the last line that might be the image ID
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "sha256:") {
			return line
		}
	}
	return ""
}

// analyzeResultAdapter adapts AnalyzeResult to ml.RepositoryAnalysis interface
type analyzeResultAdapter struct {
	*AnalyzeResult
}

func (a *analyzeResultAdapter) GetLanguage() string  { return a.Language }
func (a *analyzeResultAdapter) GetFramework() string { return a.Framework }

func (a *analyzeResultAdapter) GetDependencies() []string {
	if deps, ok := a.Analysis["dependencies"].([]interface{}); ok {
		result := make([]string, 0, len(deps))
		for _, dep := range deps {
			if s, ok := dep.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return []string{}
}

func (a *analyzeResultAdapter) GetBuildCommand() string {
	if cmd, ok := a.Analysis["build_command"].(string); ok {
		return cmd
	}
	return ""
}

func (a *analyzeResultAdapter) GetStartCommand() string {
	if cmd, ok := a.Analysis["start_command"].(string); ok {
		return cmd
	}
	return ""
}
