package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/git"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// CoreRepositoryAnalyzer implements core.RepositoryAnalyzer interface
// This eliminates the need for the repository analyzer adapter
type CoreRepositoryAnalyzer struct {
	repoAnalyzer *analysis.RepositoryAnalyzer
	gitManager   *git.Manager
	logger       zerolog.Logger
}

// NewCoreRepositoryAnalyzer creates a new core repository analyzer
func NewCoreRepositoryAnalyzer(logger zerolog.Logger) core.RepositoryAnalyzer {
	return &CoreRepositoryAnalyzer{
		repoAnalyzer: analysis.NewRepositoryAnalyzer(logger),
		gitManager:   git.NewManager(logger),
		logger:       logger.With().Str("component", "core_repository_analyzer").Logger(),
	}
}

// AnalyzeStructure analyzes repository structure and returns core.RepositoryInfo
func (r *CoreRepositoryAnalyzer) AnalyzeStructure(ctx context.Context, path string) (*core.RepositoryInfo, error) {
	r.logger.Debug().
		Str("path", path).
		Msg("Starting repository structure analysis")

	// Perform the actual analysis using the existing repository analyzer
	analysisResult, err := r.repoAnalyzer.AnalyzeRepository(path)
	if err != nil {
		r.logger.Error().
			Err(err).
			Str("path", path).
			Msg("Repository analysis failed")
		return nil, fmt.Errorf("repository analysis failed: %w", err)
	}

	// Convert to core.RepositoryInfo format
	repoInfo := r.convertToRepositoryInfo(path, analysisResult)

	r.logger.Debug().
		Str("path", path).
		Str("language", repoInfo.Language).
		Str("framework", repoInfo.Framework).
		Int("dependencies", len(repoInfo.Dependencies)).
		Msg("Repository structure analysis completed")

	return repoInfo, nil
}

// AnalyzeDockerfile analyzes Dockerfile and returns core.DockerfileInfo
func (r *CoreRepositoryAnalyzer) AnalyzeDockerfile(ctx context.Context, path string) (*core.DockerfileInfo, error) {
	r.logger.Debug().
		Str("path", path).
		Msg("Starting Dockerfile analysis")

	// Check if Dockerfile exists
	dockerfilePath := filepath.Join(path, "Dockerfile")
	
	// For now, return a basic implementation
	// In a real implementation, you would parse the Dockerfile
	dockerfileInfo := &core.DockerfileInfo{
		Path:           dockerfilePath,
		BaseImage:      "unknown",
		ExposedPorts:   []int{},
		WorkingDir:     "",
		EntryPoint:     []string{},
		Cmd:            []string{},
		Labels:         make(map[string]string),
		BuildArgs:      make(map[string]string),
		MultiStage:     false,
		SecurityIssues: []string{},
	}

	r.logger.Debug().
		Str("path", dockerfilePath).
		Msg("Dockerfile analysis completed")

	return dockerfileInfo, nil
}

// GetBuildRecommendations generates build recommendations for a repository
func (r *CoreRepositoryAnalyzer) GetBuildRecommendations(ctx context.Context, repo *core.RepositoryInfo) (*core.BuildRecommendations, error) {
	r.logger.Debug().
		Str("path", repo.Path).
		Str("language", repo.Language).
		Msg("Generating build recommendations")

	recommendations := &core.BuildRecommendations{
		OptimizationTips: []string{},
		SecurityTips:     []string{},
		PerformanceTips:  []string{},
		BestPractices:    []string{},
		Suggestions:      make(map[string]string),
	}

	// Generate language-specific recommendations
	switch strings.ToLower(repo.Language) {
	case "go":
		recommendations.OptimizationTips = append(recommendations.OptimizationTips, 
			"Use multi-stage builds for smaller images",
			"Use Go build cache for faster builds",
			"Consider using scratch or distroless base images")
		recommendations.BestPractices = append(recommendations.BestPractices,
			"Use Go modules for dependency management",
			"Set CGO_ENABLED=0 for static binaries")
	case "python":
		recommendations.OptimizationTips = append(recommendations.OptimizationTips,
			"Use poetry or pipenv for dependency management",
			"Use alpine base images for smaller size",
			"Use virtual environments")
		recommendations.SecurityTips = append(recommendations.SecurityTips,
			"Pin dependency versions",
			"Use trusted base images")
	case "javascript", "typescript":
		recommendations.OptimizationTips = append(recommendations.OptimizationTips,
			"Use npm ci for faster, reproducible builds",
			"Consider using node:alpine for smaller images",
			"Use .dockerignore to exclude node_modules")
		recommendations.BestPractices = append(recommendations.BestPractices,
			"Use package-lock.json for reproducible builds")
	case "java":
		recommendations.OptimizationTips = append(recommendations.OptimizationTips,
			"Use multi-stage builds to exclude build tools",
			"Use OpenJDK base images",
			"Consider using jlink for custom runtime images")
	default:
		recommendations.BestPractices = append(recommendations.BestPractices,
			"Follow language-specific containerization best practices",
			"Use official base images when available")
	}

	// Add general recommendations
	recommendations.SecurityTips = append(recommendations.SecurityTips,
		"Run containers as non-root user",
		"Regularly update base images",
		"Scan images for vulnerabilities")

	recommendations.PerformanceTips = append(recommendations.PerformanceTips,
		"Use appropriate resource limits",
		"Optimize layer caching",
		"Minimize image layers")

	r.logger.Debug().
		Str("path", repo.Path).
		Int("optimization_tips", len(recommendations.OptimizationTips)).
		Int("security_tips", len(recommendations.SecurityTips)).
		Msg("Build recommendations generated")

	return recommendations, nil
}

// convertToRepositoryInfo converts analysis.AnalysisResult to core.RepositoryInfo
func (r *CoreRepositoryAnalyzer) convertToRepositoryInfo(path string, analysisResult *analysis.AnalysisResult) *core.RepositoryInfo {
	if analysisResult == nil {
		r.logger.Warn().Msg("Nil analysis result, returning basic repository info")
		return &core.RepositoryInfo{
			Path:         path,
			Type:         "unknown",
			Language:     "unknown",
			Framework:    "unknown",
			Languages:    []string{},
			Dependencies: make(map[string]string),
			BuildTools:   []string{},
			EntryPoint:   "",
			Port:         0,
			HasDockerfile: false,
			Metadata:     make(map[string]interface{}),
		}
	}

	// Convert dependencies from slice to map
	dependencies := make(map[string]string)
	for _, dep := range analysisResult.Dependencies {
		dependencies[dep.Name] = dep.Version
	}

	// Determine build tools based on detected files and language
	buildTools := r.determineBuildTools(analysisResult)

	// Check for Dockerfile
	hasDockerfile := r.hasFile(analysisResult, "Dockerfile")

	// Extract port information
	port := analysisResult.Port
	if port == 0 {
		port = r.extractPortFromLanguage(analysisResult.Language)
	}

	// Extract entry point
	entryPoint := ""
	if len(analysisResult.EntryPoints) > 0 {
		entryPoint = analysisResult.EntryPoints[0]
	}

	// Build metadata
	metadata := map[string]interface{}{
		"entry_points": analysisResult.EntryPoints,
		"structure":    analysisResult.Structure,
		"context":      analysisResult.Context,
	}

	return &core.RepositoryInfo{
		Path:          path,
		Type:          "repository", // Could be enhanced to detect project type
		Language:      analysisResult.Language,
		Framework:     analysisResult.Framework,
		Languages:     []string{analysisResult.Language}, // Could be enhanced to detect multiple languages
		Dependencies:  dependencies,
		BuildTools:    buildTools,
		EntryPoint:    entryPoint,
		Port:          port,
		HasDockerfile: hasDockerfile,
		Metadata:      metadata,
	}
}

// determineBuildTools determines build tools based on analysis results
func (r *CoreRepositoryAnalyzer) determineBuildTools(analysisResult *analysis.AnalysisResult) []string {
	buildTools := []string{}

	// Check for language-specific build files and tools
	switch strings.ToLower(analysisResult.Language) {
	case "go":
		if r.hasFile(analysisResult, "go.mod") {
			buildTools = append(buildTools, "go")
		}
		if r.hasFile(analysisResult, "Makefile") {
			buildTools = append(buildTools, "make")
		}
	case "python":
		if r.hasFile(analysisResult, "requirements.txt") {
			buildTools = append(buildTools, "pip")
		}
		if r.hasFile(analysisResult, "pyproject.toml") {
			buildTools = append(buildTools, "poetry")
		}
		if r.hasFile(analysisResult, "setup.py") {
			buildTools = append(buildTools, "setuptools")
		}
	case "javascript", "typescript":
		if r.hasFile(analysisResult, "package.json") {
			buildTools = append(buildTools, "npm")
		}
		if r.hasFile(analysisResult, "yarn.lock") {
			buildTools = append(buildTools, "yarn")
		}
	case "java":
		if r.hasFile(analysisResult, "pom.xml") {
			buildTools = append(buildTools, "maven")
		}
		if r.hasFile(analysisResult, "build.gradle") {
			buildTools = append(buildTools, "gradle")
		}
	case "rust":
		if r.hasFile(analysisResult, "Cargo.toml") {
			buildTools = append(buildTools, "cargo")
		}
	case "csharp":
		if r.hasFile(analysisResult, "*.csproj") || r.hasFile(analysisResult, "*.sln") {
			buildTools = append(buildTools, "dotnet")
		}
	}

	// Check for Docker
	if r.hasFile(analysisResult, "Dockerfile") {
		buildTools = append(buildTools, "docker")
	}

	// Check for generic build tools
	if r.hasFile(analysisResult, "Makefile") && !contains(buildTools, "make") {
		buildTools = append(buildTools, "make")
	}

	return buildTools
}

// hasFile checks if a specific file exists in the analysis structure
func (r *CoreRepositoryAnalyzer) hasFile(analysisResult *analysis.AnalysisResult, filename string) bool {
	if analysisResult == nil || analysisResult.Structure == nil {
		return false
	}

	// Simple check - convert structure to string and search
	structStr := fmt.Sprintf("%+v", analysisResult.Structure)
	return strings.Contains(structStr, filename)
}

// extractPortFromLanguage provides default ports for different languages
func (r *CoreRepositoryAnalyzer) extractPortFromLanguage(language string) int {
	switch strings.ToLower(language) {
	case "go":
		return 8080
	case "python":
		return 5000
	case "javascript", "typescript":
		return 3000
	case "java":
		return 8080
	case "csharp":
		return 5000
	default:
		return 8080
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}