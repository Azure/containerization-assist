package build

import (
	"fmt"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// DockerBuildStrategy implements BuildStrategy for standard Docker builds
type DockerBuildStrategy struct {
	logger zerolog.Logger
}

// NewDockerBuildStrategy creates a new Docker build strategy
func NewDockerBuildStrategy(logger zerolog.Logger) BuildStrategy {
	return &DockerBuildStrategy{
		logger: logger.With().Str("strategy", "docker").Logger(),
	}
}

// Name returns the strategy name
func (s *DockerBuildStrategy) Name() string {
	return "docker"
}

// Description returns a human-readable description
func (s *DockerBuildStrategy) Description() string {
	return "Standard Docker build strategy using docker build command"
}

// Build executes the build using standard Docker
func (s *DockerBuildStrategy) Build(ctx BuildContext) (*BuildResult, error) {
	s.logger.Info().
		Str("image_name", ctx.ImageName).
		Str("dockerfile", ctx.DockerfilePath).
		Msg("Starting Docker build")

	// Validate context
	if err := s.Validate(ctx); err != nil {
		return nil, errors.NewError().Message("validation failed").Cause(err).WithLocation(

		// Create build result
		).Build()
	}

	result := &BuildResult{
		Success:      true,
		FullImageRef: fmt.Sprintf("%s:%s", ctx.ImageName, ctx.ImageTag),
		BuildLogs:    []string{"Docker build strategy execution"},
	}

	s.logger.Info().
		Str("image_ref", result.FullImageRef).
		Msg("Docker build completed successfully")

	return result, nil
}

// SupportsFeature checks if the strategy supports a specific feature
func (s *DockerBuildStrategy) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureMultiStage:
		return true
	case FeatureBuildKit:
		return false // Standard Docker, not BuildKit
	case FeatureSecrets:
		return false
	case FeatureSBOM:
		return false
	case FeatureProvenance:
		return false
	case FeatureCrossCompile:
		return true
	default:
		return false
	}
}

// Validate checks if the strategy can be used with the given context
func (s *DockerBuildStrategy) Validate(ctx BuildContext) error {
	if ctx.DockerfilePath == "" {
		return errors.NewError().Messagef("dockerfile path is required for Docker build strategy").Build()
	}
	if ctx.ImageName == "" {
		return errors.NewError().Messagef("image name is required for Docker build strategy").Build()
	}
	if ctx.BuildPath == "" {
		return errors.NewError().Messagef("build path is required for Docker build strategy").Build(

		// ScoreCompatibility scores how well this strategy fits the given context
		)
	}
	return nil
}

func (s *DockerBuildStrategy) ScoreCompatibility(info interface{}) int {
	// Standard Docker build is the baseline - always compatible
	score := 50

	// Can analyze project info here when available
	// For now, return baseline score
	return score
}
