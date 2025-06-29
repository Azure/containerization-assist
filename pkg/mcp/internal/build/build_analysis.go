package build

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

// BuildAnalyzer handles build context analysis and validation
type BuildAnalyzer struct {
	logger zerolog.Logger
}

// NewBuildAnalyzer creates a new build analyzer
func NewBuildAnalyzer(logger zerolog.Logger) *BuildAnalyzer {
	return &BuildAnalyzer{
		logger: logger.With().Str("component", "build_analyzer").Logger(),
	}
}

// AnalyzeBuildContext analyzes the build context and populates result info
func (a *BuildAnalyzer) AnalyzeBuildContext(result *AtomicBuildImageResult) error {
	a.logger.Debug().
		Str("dockerfile_path", result.DockerfilePath).
		Str("build_context", result.BuildContext).
		Msg("Starting build context analysis")
	// Check if Dockerfile exists
	if _, err := os.Stat(result.DockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found at path: %s", result.DockerfilePath)
	} else if err != nil {
		return fmt.Errorf("failed to check dockerfile: %w", err)
	}

	return nil
}
