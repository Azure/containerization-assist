package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		return fmt.Errorf("dockerfile not found at %s", result.DockerfilePath)
	}

	// Analyze the Dockerfile
	if err := a.analyzeDockerfile(result); err != nil {
		a.logger.Warn().Err(err).Msg("Dockerfile analysis failed")
		// Don't fail the build, just log warning
	}

	// Analyze build context directory
	if err := a.AnalyzeBuildContextDirectory(result); err != nil {
		a.logger.Warn().Err(err).Msg("Build context directory analysis failed")
		// Don't fail the build, just log warning
	}

	a.logger.Debug().Msg("Build context analysis completed")
	return nil
}

// analyzeDockerfile analyzes the Dockerfile content
func (a *BuildAnalyzer) analyzeDockerfile(result *AtomicBuildImageResult) error {
	content, err := os.ReadFile(result.DockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to read dockerfile: %w", err)
	}

	dockerfileStr := string(content)
	result.BuildContext_Info.DockerfileLines = len(strings.Split(dockerfileStr, "\n"))

	// Extract base image
	lines := strings.Split(dockerfileStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				result.BuildContext_Info.BaseImage = parts[1]
				break
			}
		}
	}

	return nil
}

// AnalyzeBuildContextDirectory analyzes the build context directory
func (a *BuildAnalyzer) AnalyzeBuildContextDirectory(result *AtomicBuildImageResult) error {
	a.logger.Debug().Str("build_context", result.BuildContext).Msg("Analyzing build context directory")

	contextInfo := &BuildContextInfo{}
	result.BuildContext_Info = contextInfo

	// Check for .dockerignore
	dockerignorePath := filepath.Join(result.BuildContext, ".dockerignore")
	if _, err := os.Stat(dockerignorePath); err == nil {
		contextInfo.HasDockerIgnore = true
		a.logger.Debug().Msg("Found .dockerignore file")
	}

	// Analyze directory structure
	totalSize := int64(0)
	fileCount := 0
	largeFiles := []string{}

	err := filepath.Walk(result.BuildContext, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileCount++
			totalSize += info.Size()

			// Track large files (>10MB)
			if info.Size() > 10*1024*1024 {
				relPath, _ := filepath.Rel(result.BuildContext, path)
				largeFiles = append(largeFiles, fmt.Sprintf("%s (%d MB)", relPath, info.Size()/(1024*1024)))
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to analyze build context directory: %w", err)
	}

	contextInfo.ContextSize = totalSize
	contextInfo.FileCount = fileCount
	contextInfo.LargeFilesFound = largeFiles

	a.logger.Debug().
		Int64("total_size", totalSize).
		Int("file_count", fileCount).
		Int("large_files", len(largeFiles)).
		Msg("Build context analysis completed")

	return nil
}

// ValidateBuildPrerequisites validates that all build prerequisites are met
func (a *BuildAnalyzer) ValidateBuildPrerequisites(result *AtomicBuildImageResult) error {
	a.logger.Debug().Msg("Validating build prerequisites")

	// Check if Docker daemon is accessible
	// This would typically involve calling docker info or similar
	// For now, we'll assume it's available

	// Validate Dockerfile exists and is readable
	if _, err := os.Stat(result.DockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found: %s", result.DockerfilePath)
	}

	// Validate build context is accessible
	if _, err := os.Stat(result.BuildContext); os.IsNotExist(err) {
		return fmt.Errorf("build context directory not found: %s", result.BuildContext)
	}

	a.logger.Debug().Msg("Build prerequisites validation completed")
	return nil
}

// GenerateBuildContext populates build context information
func (a *BuildAnalyzer) GenerateBuildContext(result *AtomicBuildImageResult) {
	if result.BuildContext_Info == nil {
		result.BuildContext_Info = &BuildContextInfo{}
	}

	// Add common suggestions
	result.BuildContext_Info.NextStepSuggestions = []string{
		"Build completed successfully",
		fmt.Sprintf("Image available as: %s", result.FullImageRef),
		"Consider running security scan on the built image",
		"Review build logs for any warnings or optimization opportunities",
	}

	// Add context-specific suggestions based on analysis
	if !result.BuildContext_Info.HasDockerIgnore {
		result.BuildContext_Info.NextStepSuggestions = append(
			result.BuildContext_Info.NextStepSuggestions,
			"Consider adding .dockerignore to optimize build context",
		)
	}

	if len(result.BuildContext_Info.LargeFilesFound) > 0 {
		result.BuildContext_Info.NextStepSuggestions = append(
			result.BuildContext_Info.NextStepSuggestions,
			"Review large files in build context for optimization opportunities",
		)
	}

	if strings.Contains(strings.ToLower(result.BuildContext_Info.BaseImage), "latest") {
		result.BuildContext_Info.NextStepSuggestions = append(
			result.BuildContext_Info.NextStepSuggestions,
			"Consider pinning base image version instead of using 'latest'",
		)
	}
}
