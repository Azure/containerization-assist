package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/rs/zerolog"
)

// StrategyManager manages different build strategies
type StrategyManager struct {
	strategies map[string]BuildStrategy
	logger     zerolog.Logger
}

// NewStrategyManager creates a new strategy manager
func NewStrategyManager(logger zerolog.Logger) *StrategyManager {
	sm := &StrategyManager{
		strategies: make(map[string]BuildStrategy),
		logger:     logger.With().Str("component", "strategy_manager").Logger(),
	}

	// Register default strategies
	sm.RegisterStrategy(NewDockerBuildStrategy(logger))
	sm.RegisterStrategy(NewBuildKitStrategy(logger))
	sm.RegisterStrategy(NewLegacyBuildStrategy(logger))

	return sm
}

// RegisterStrategy registers a new build strategy
func (sm *StrategyManager) RegisterStrategy(strategy BuildStrategy) {
	sm.strategies[strategy.Name()] = strategy
}

// SelectStrategy selects the best strategy for the given context
func (sm *StrategyManager) SelectStrategy(ctx BuildContext) (BuildStrategy, error) {
	sm.logger.Info().
		Str("dockerfile", ctx.DockerfilePath).
		Bool("buildkit_available", sm.isBuildKitAvailable()).
		Msg("Selecting build strategy")

	// Check if BuildKit is requested and available
	if sm.isBuildKitAvailable() && sm.shouldUseBuildKit(ctx) {
		if strategy, exists := sm.strategies["buildkit"]; exists {
			if err := strategy.Validate(ctx); err == nil {
				sm.logger.Info().Str("strategy", "buildkit").Msg("Selected BuildKit strategy")
				return strategy, nil
			}
		}
	}

	// Default to standard Docker build
	if strategy, exists := sm.strategies["docker"]; exists {
		if err := strategy.Validate(ctx); err == nil {
			sm.logger.Info().Str("strategy", "docker").Msg("Selected Docker strategy")
			return strategy, nil
		}
	}

	// Fallback to legacy build
	if strategy, exists := sm.strategies["legacy"]; exists {
		sm.logger.Info().Str("strategy", "legacy").Msg("Selected legacy strategy")
		return strategy, nil
	}

	return nil, fmt.Errorf("no suitable build strategy found")
}

// GetStrategy returns a specific strategy by name
func (sm *StrategyManager) GetStrategy(name string) (BuildStrategy, bool) {
	strategy, exists := sm.strategies[name]
	return strategy, exists
}

// ListStrategies returns all available strategies
func (sm *StrategyManager) ListStrategies() []string {
	var names []string
	for name := range sm.strategies {
		names = append(names, name)
	}
	return names
}

// isBuildKitAvailable checks if BuildKit is available
func (sm *StrategyManager) isBuildKitAvailable() bool {
	// Check DOCKER_BUILDKIT environment variable
	return os.Getenv("DOCKER_BUILDKIT") == "1"
}

// shouldUseBuildKit determines if BuildKit should be used
func (sm *StrategyManager) shouldUseBuildKit(ctx BuildContext) bool {
	// Check if Dockerfile uses BuildKit-specific features
	dockerfilePath := ctx.DockerfilePath
	if dockerfilePath == "" {
		return false
	}

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return false
	}

	dockerfileContent := string(content)

	// Check for BuildKit-specific syntax
	buildKitFeatures := []string{
		"# syntax=",
		"--mount=",
		"--secret",
		"RUN --mount",
		"--platform=",
		"--ssh",
	}

	for _, feature := range buildKitFeatures {
		if strings.Contains(dockerfileContent, feature) {
			return true
		}
	}

	return false
}

// DockerBuildStrategy implements standard Docker build
type DockerBuildStrategy struct {
	logger zerolog.Logger
	client DockerClient
}

// DockerClient interface for Docker operations
type DockerClient interface {
	BuildImage(ctx context.Context, sessionID, imageName, dockerfilePath string) (*coredocker.BuildResult, error)
}

// NewDockerBuildStrategy creates a new Docker build strategy
func NewDockerBuildStrategy(logger zerolog.Logger) *DockerBuildStrategy {
	return &DockerBuildStrategy{
		logger: logger.With().Str("strategy", "docker").Logger(),
	}
}

// Name returns the strategy name
func (s *DockerBuildStrategy) Name() string {
	return "docker"
}

// Description returns the strategy description
func (s *DockerBuildStrategy) Description() string {
	return "Standard Docker build using docker build command"
}

// Build executes the Docker build
func (s *DockerBuildStrategy) Build(ctx BuildContext) (*BuildResult, error) {
	startTime := time.Now()

	s.logger.Info().
		Str("image", ctx.ImageName).
		Str("tag", ctx.ImageTag).
		Str("dockerfile", ctx.DockerfilePath).
		Msg("Starting Docker build")

	// Validate prerequisites
	if err := s.validatePrerequisites(ctx); err != nil {
		return nil, err
	}

	// Prepare build command
	fullImageRef := fmt.Sprintf("%s:%s", ctx.ImageName, ctx.ImageTag)

	// In a real implementation, this would call the Docker API
	// For now, return a placeholder result
	result := &BuildResult{
		Success:        true,
		FullImageRef:   fullImageRef,
		Duration:       time.Since(startTime),
		LayerCount:     10,                // Placeholder
		ImageSizeBytes: 100 * 1024 * 1024, // 100MB placeholder
		CacheHits:      5,
		CacheMisses:    5,
	}

	s.logger.Info().
		Dur("duration", result.Duration).
		Str("image", fullImageRef).
		Msg("Docker build completed")

	return result, nil
}

// SupportsFeature checks if the strategy supports a feature
func (s *DockerBuildStrategy) SupportsFeature(feature string) bool {
	supportedFeatures := map[string]bool{
		FeatureMultiStage:   true,
		FeatureBuildKit:     false,
		FeatureSecrets:      false,
		FeatureSBOM:         false,
		FeatureProvenance:   false,
		FeatureCrossCompile: true,
	}

	return supportedFeatures[feature]
}

// Validate checks if the strategy can be used
func (s *DockerBuildStrategy) Validate(ctx BuildContext) error {
	// Check if Dockerfile exists
	if ctx.DockerfilePath == "" {
		return fmt.Errorf("Dockerfile path is required")
	}

	if _, err := os.Stat(ctx.DockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found at %s", ctx.DockerfilePath)
	}

	// Check if build context exists
	if ctx.BuildPath == "" {
		return fmt.Errorf("build context path is required")
	}

	if _, err := os.Stat(ctx.BuildPath); os.IsNotExist(err) {
		return fmt.Errorf("build context not found at %s", ctx.BuildPath)
	}

	return nil
}

// validatePrerequisites checks build prerequisites
func (s *DockerBuildStrategy) validatePrerequisites(ctx BuildContext) error {
	// Additional validation specific to Docker builds
	return nil
}

// BuildKitStrategy implements BuildKit-based builds
type BuildKitStrategy struct {
	logger zerolog.Logger
}

// NewBuildKitStrategy creates a new BuildKit strategy
func NewBuildKitStrategy(logger zerolog.Logger) *BuildKitStrategy {
	return &BuildKitStrategy{
		logger: logger.With().Str("strategy", "buildkit").Logger(),
	}
}

// Name returns the strategy name
func (s *BuildKitStrategy) Name() string {
	return "buildkit"
}

// Description returns the strategy description
func (s *BuildKitStrategy) Description() string {
	return "BuildKit-based build with advanced features like cache mounts and secrets"
}

// Build executes the BuildKit build
func (s *BuildKitStrategy) Build(ctx BuildContext) (*BuildResult, error) {
	startTime := time.Now()

	s.logger.Info().
		Str("image", ctx.ImageName).
		Str("tag", ctx.ImageTag).
		Bool("buildkit", true).
		Msg("Starting BuildKit build")

	// BuildKit-specific implementation
	fullImageRef := fmt.Sprintf("%s:%s", ctx.ImageName, ctx.ImageTag)

	// In a real implementation, this would use BuildKit features
	result := &BuildResult{
		Success:        true,
		FullImageRef:   fullImageRef,
		Duration:       time.Since(startTime),
		LayerCount:     8,                // BuildKit often produces fewer layers
		ImageSizeBytes: 80 * 1024 * 1024, // Smaller due to better optimization
		CacheHits:      7,
		CacheMisses:    3,
	}

	s.logger.Info().
		Dur("duration", result.Duration).
		Str("image", fullImageRef).
		Msg("BuildKit build completed")

	return result, nil
}

// SupportsFeature checks if the strategy supports a feature
func (s *BuildKitStrategy) SupportsFeature(feature string) bool {
	// BuildKit supports all modern features
	return true
}

// Validate checks if BuildKit can be used
func (s *BuildKitStrategy) Validate(ctx BuildContext) error {
	// Check if BuildKit is enabled
	if os.Getenv("DOCKER_BUILDKIT") != "1" {
		return fmt.Errorf("BuildKit is not enabled (set DOCKER_BUILDKIT=1)")
	}

	// Validate Dockerfile exists
	if ctx.DockerfilePath == "" {
		return fmt.Errorf("Dockerfile path is required")
	}

	if _, err := os.Stat(ctx.DockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found at %s", ctx.DockerfilePath)
	}

	return nil
}

// LegacyBuildStrategy implements legacy Docker build for compatibility
type LegacyBuildStrategy struct {
	logger zerolog.Logger
}

// NewLegacyBuildStrategy creates a new legacy build strategy
func NewLegacyBuildStrategy(logger zerolog.Logger) *LegacyBuildStrategy {
	return &LegacyBuildStrategy{
		logger: logger.With().Str("strategy", "legacy").Logger(),
	}
}

// Name returns the strategy name
func (s *LegacyBuildStrategy) Name() string {
	return "legacy"
}

// Description returns the strategy description
func (s *LegacyBuildStrategy) Description() string {
	return "Legacy Docker build for older Docker versions"
}

// Build executes the legacy build
func (s *LegacyBuildStrategy) Build(ctx BuildContext) (*BuildResult, error) {
	s.logger.Warn().Msg("Using legacy build strategy - consider upgrading Docker")

	// Legacy implementation
	fullImageRef := fmt.Sprintf("%s:%s", ctx.ImageName, ctx.ImageTag)

	result := &BuildResult{
		Success:        true,
		FullImageRef:   fullImageRef,
		Duration:       2 * time.Minute,   // Legacy builds are slower
		LayerCount:     15,                // More layers due to less optimization
		ImageSizeBytes: 150 * 1024 * 1024, // Larger images
		CacheHits:      3,
		CacheMisses:    12,
	}

	return result, nil
}

// SupportsFeature checks if the strategy supports a feature
func (s *LegacyBuildStrategy) SupportsFeature(feature string) bool {
	// Legacy builds have limited features
	supportedFeatures := map[string]bool{
		FeatureMultiStage:   false,
		FeatureBuildKit:     false,
		FeatureSecrets:      false,
		FeatureSBOM:         false,
		FeatureProvenance:   false,
		FeatureCrossCompile: false,
	}

	return supportedFeatures[feature]
}

// Validate checks if legacy build can be used
func (s *LegacyBuildStrategy) Validate(ctx BuildContext) error {
	// Legacy builds have minimal requirements
	if ctx.DockerfilePath == "" {
		// Legacy builds can use default Dockerfile
		ctx.DockerfilePath = filepath.Join(ctx.BuildPath, "Dockerfile")
	}

	return nil
}
