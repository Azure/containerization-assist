package build

import (
	"os"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// BuildAnalyzer handles build context analysis and validation
type BuildAnalyzer struct {
	logger *slog.Logger
}

// NewBuildAnalyzer creates a new build analyzer
func NewBuildAnalyzer(logger *slog.Logger) *BuildAnalyzer {
	return &BuildAnalyzer{
		logger: logger.With("component", "build_analyzer"),
	}
}

// AnalyzeBuildContext analyzes the build context and populates result info
func (a *BuildAnalyzer) AnalyzeBuildContext(result *AtomicBuildImageResult) error {
	a.logger.Debug("Starting build context analysis",
		"dockerfile_path", result.DockerfilePath,
		"build_context", result.BuildContext)
	// Check if Dockerfile exists
	if _, err := os.Stat(result.DockerfilePath); os.IsNotExist(err) {
		return errors.NewError().Messagef("dockerfile not found at path: %s", result.DockerfilePath).Build()
	} else if err != nil {
		return errors.NewError().Message("failed to check dockerfile").Cause(err).Build()
	}

	return nil
}
