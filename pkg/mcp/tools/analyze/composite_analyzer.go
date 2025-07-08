package analyze

import (
	"context"
	"fmt"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// UnifiedAnalyzer implements the unified core.Analyzer interface by combining
// repository analysis and AI analysis capabilities
type UnifiedAnalyzer struct {
	repoAnalyzer *analysis.RepositoryAnalyzer
	aiAnalyzer   core.AIAnalyzer
	logger       *slog.Logger
}

// NewUnifiedAnalyzer creates a new unified analyzer that implements services.Analyzer
func NewUnifiedAnalyzer(repoAnalyzer *analysis.RepositoryAnalyzer, aiAnalyzer core.AIAnalyzer, logger *slog.Logger) services.Analyzer {
	return &UnifiedAnalyzer{
		repoAnalyzer: repoAnalyzer,
		aiAnalyzer:   aiAnalyzer,
		logger:       logger.With("component", "unified_analyzer"),
	}
}

// Repository analysis methods (from RepositoryAnalyzer)

func (u *UnifiedAnalyzer) AnalyzeStructure(ctx context.Context, path string) (*core.RepositoryInfo, error) {
	analysisResult, err := u.repoAnalyzer.AnalyzeRepository(path)
	if err != nil {
		return nil, err
	}
	
	return &core.RepositoryInfo{
		Path:           path,
		Name:           "repository",
		Language:       analysisResult.Language,
		Framework:      analysisResult.Framework,
		Dependencies:   convertDepsToStringSlice(analysisResult.Dependencies),
		BuildTool:      getBuildToolFromList(extractBuildTools(analysisResult.ConfigFiles)),
		PackageManager: getPackageManagerFromDeps(analysisResult.Dependencies),
		Metadata:       make(map[string]string),
	}, nil
}

func (u *UnifiedAnalyzer) AnalyzeDockerfile(ctx context.Context, path string) (*core.DockerfileInfo, error) {
	// Simple implementation - return basic info
	return &core.DockerfileInfo{
		Path:         path,
		BaseImage:    "unknown",
		Instructions: []string{},
		ExposedPorts: []string{},
		WorkDir:      "/app",
		User:         "root",
		Metadata:     make(map[string]string),
	}, nil
}

func (u *UnifiedAnalyzer) GetBuildRecommendations(ctx context.Context, repo *core.RepositoryInfo) (*core.BuildRecommendations, error) {
	// Generate basic recommendations based on language
	return &core.BuildRecommendations{
		OptimizationSuggestions: []core.Recommendation{
			{Type: "optimization", Priority: 1, Title: "Use multi-stage builds", Description: "Reduce image size", Action: "implement", Metadata: make(map[string]string)},
		},
		SecurityRecommendations: []core.Recommendation{
			{Type: "security", Priority: 1, Title: "Use non-root user", Description: "Improve security", Action: "add_user", Metadata: make(map[string]string)},
		},
	}, nil
}

// AI analysis methods (from AIAnalyzer)

func (u *UnifiedAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return u.aiAnalyzer.Analyze(ctx, prompt)
}

func (u *UnifiedAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	// Use available AnalyzeWithContext method
	context := map[string]interface{}{
		"base_dir": baseDir,
		"mode":     "file_tools",
	}
	return u.aiAnalyzer.AnalyzeWithContext(ctx, prompt, context)
}

func (u *UnifiedAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	// Format the prompt and use basic Analyze method
	prompt := fmt.Sprintf(promptTemplate, args...)
	return u.aiAnalyzer.Analyze(ctx, prompt)
}

func (u *UnifiedAnalyzer) GetTokenUsage() core.TokenUsage {
	// Return empty token usage since the interface doesn't provide this
	return core.TokenUsage{
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
	}
}

func (u *UnifiedAnalyzer) ResetTokenUsage() {
	// No-op since the interface doesn't provide this
}

// Implement services.Analyzer interface methods

// AnalyzeRepository analyzes a repository and returns a services.AnalysisResult
func (u *UnifiedAnalyzer) AnalyzeRepository(ctx context.Context, path string) (*services.AnalysisResult, error) {
	// Use the repository analyzer to get detailed analysis
	analysisResult, err := u.repoAnalyzer.AnalyzeRepository(path)
	if err != nil {
		return nil, err
	}

	// Convert analysis.AnalysisResult to services.AnalysisResult
	entryPoint := ""
	if len(analysisResult.EntryPoints) > 0 {
		entryPoint = analysisResult.EntryPoints[0]
	}

	result := &services.AnalysisResult{
		Language:     analysisResult.Language,
		Framework:    analysisResult.Framework,
		Dependencies: convertDepsToStringSlice(analysisResult.Dependencies),
		EntryPoint:   entryPoint,
		Port:         analysisResult.Port,
		BuildCommand: "", // Not available in analysis result
		RunCommand:   "", // Not available in analysis result
	}

	return result, nil
}

// DetectFramework detects the framework used in the repository
func (u *UnifiedAnalyzer) DetectFramework(ctx context.Context, path string) (string, error) {
	analysisResult, err := u.repoAnalyzer.AnalyzeRepository(path)
	if err != nil {
		return "", err
	}
	return analysisResult.Framework, nil
}

// GenerateDockerfile generates a Dockerfile based on analysis results
func (u *UnifiedAnalyzer) GenerateDockerfile(ctx context.Context, analysis *services.AnalysisResult) (string, error) {
	// Use AI analyzer to generate Dockerfile based on analysis
	prompt := fmt.Sprintf(
		"Generate a Dockerfile for a %s application using %s framework. "+
			"Dependencies: %v, Entry point: %s, Port: %d, Build command: %s, Run command: %s",
		analysis.Language, analysis.Framework, analysis.Dependencies,
		analysis.EntryPoint, analysis.Port, analysis.BuildCommand, analysis.RunCommand,
	)

	return u.aiAnalyzer.Analyze(ctx, prompt)
}
