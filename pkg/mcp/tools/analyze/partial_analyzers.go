package analyze

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// PartialRepositoryAnalyzer implements full Analyzer interface but only provides repository functionality
// AI methods return not-implemented errors
type PartialRepositoryAnalyzer struct {
	repo *analysis.RepositoryAnalyzer
}

// NewPartialRepositoryAnalyzer creates an Analyzer that only supports repository analysis
func NewPartialRepositoryAnalyzer(repo *analysis.RepositoryAnalyzer) services.Analyzer {
	return &PartialRepositoryAnalyzer{repo: repo}
}

// Repository analysis methods - delegate to wrapped analyzer

func (p *PartialRepositoryAnalyzer) AnalyzeStructure(ctx context.Context, path string) (*core.RepositoryInfo, error) {
	return p.repo.AnalyzeStructure(ctx, path)
}

func (p *PartialRepositoryAnalyzer) AnalyzeDockerfile(ctx context.Context, path string) (*core.DockerfileInfo, error) {
	return p.repo.AnalyzeDockerfile(ctx, path)
}

func (p *PartialRepositoryAnalyzer) GetBuildRecommendations(ctx context.Context, repo *core.RepositoryInfo) (*core.BuildRecommendations, error) {
	return p.repo.GetBuildRecommendations(ctx, repo)
}

// AI analysis methods - return not implemented errors

func (p *PartialRepositoryAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "", errors.NewError().Messagef("AI analysis not available in repository-only analyzer").Build()
}

func (p *PartialRepositoryAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "", errors.NewError().Messagef("AI analysis not available in repository-only analyzer").Build()
}

func (p *PartialRepositoryAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "", errors.NewError().Messagef("AI analysis not available in repository-only analyzer").Build()
}

func (p *PartialRepositoryAnalyzer) GetTokenUsage() core.TokenUsage {
	return core.TokenUsage{} // Return empty usage
}

func (p *PartialRepositoryAnalyzer) ResetTokenUsage() {
	// No-op - no tokens to reset
}

// Implement services.Analyzer interface methods

func (p *PartialRepositoryAnalyzer) AnalyzeRepository(ctx context.Context, path string) (*services.AnalysisResult, error) {
	repoInfo, err := p.repo.AnalyzeStructure(ctx, path)
	if err != nil {
		return nil, err
	}

	// Convert core.RepositoryInfo to services.AnalysisResult
	return &services.AnalysisResult{
		Language:     repoInfo.Language,
		Framework:    repoInfo.Framework,
		Dependencies: repoInfo.Dependencies,
		EntryPoint:   repoInfo.EntryPoint,
		Port:         repoInfo.Port,
		BuildCommand: repoInfo.BuildCommand,
		RunCommand:   repoInfo.RunCommand,
	}, nil
}

func (p *PartialRepositoryAnalyzer) DetectFramework(ctx context.Context, path string) (string, error) {
	repoInfo, err := p.repo.AnalyzeStructure(ctx, path)
	if err != nil {
		return "", err
	}
	return repoInfo.Framework, nil
}

func (p *PartialRepositoryAnalyzer) GenerateDockerfile(ctx context.Context, analysis *services.AnalysisResult) (string, error) {
	return "", errors.NewError().
		Message("Dockerfile generation not supported").
		Code("ANALYZER_NOT_IMPLEMENTED").
		Hint("This analyzer only supports repository analysis")
}

// PartialAIAnalyzer implements full Analyzer interface but only provides AI functionality
// Repository methods return not-implemented errors
type PartialAIAnalyzer struct {
	ai core.AIAnalyzer
}

// NewPartialAIAnalyzer creates an Analyzer that only supports AI analysis
func NewPartialAIAnalyzer(ai core.AIAnalyzer) services.Analyzer {
	return &PartialAIAnalyzer{ai: ai}
}

// AI analysis methods - delegate to wrapped analyzer

func (p *PartialAIAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return p.ai.Analyze(ctx, prompt)
}

func (p *PartialAIAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return p.ai.AnalyzeWithFileTools(ctx, prompt, baseDir)
}

func (p *PartialAIAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return p.ai.AnalyzeWithFormat(ctx, promptTemplate, args...)
}

func (p *PartialAIAnalyzer) GetTokenUsage() core.TokenUsage {
	return p.ai.GetTokenUsage()
}

func (p *PartialAIAnalyzer) ResetTokenUsage() {
	p.ai.ResetTokenUsage()
}

// Repository analysis methods - return not implemented errors

func (p *PartialAIAnalyzer) AnalyzeStructure(ctx context.Context, path string) (*core.RepositoryInfo, error) {
	return nil, errors.NewError().Messagef("repository analysis not available in AI-only analyzer").Build()
}

func (p *PartialAIAnalyzer) AnalyzeDockerfile(ctx context.Context, path string) (*core.DockerfileInfo, error) {
	return nil, errors.NewError().Messagef("repository analysis not available in AI-only analyzer").Build()
}

func (p *PartialAIAnalyzer) GetBuildRecommendations(ctx context.Context, repo *core.RepositoryInfo) (*core.BuildRecommendations, error) {
	return nil, errors.NewError().Messagef("repository analysis not available in AI-only analyzer").Build()
}

// Implement services.Analyzer interface methods

func (p *PartialAIAnalyzer) AnalyzeRepository(ctx context.Context, path string) (*services.AnalysisResult, error) {
	return nil, errors.NewError().
		Message("Repository analysis not supported").
		Code("ANALYZER_NOT_IMPLEMENTED").
		Hint("This analyzer only supports AI analysis")
}

func (p *PartialAIAnalyzer) DetectFramework(ctx context.Context, path string) (string, error) {
	return "", errors.NewError().
		Message("Framework detection not supported").
		Code("ANALYZER_NOT_IMPLEMENTED").
		Hint("This analyzer only supports AI analysis")
}

func (p *PartialAIAnalyzer) GenerateDockerfile(ctx context.Context, analysis *services.AnalysisResult) (string, error) {
	// This one we can implement using AI analysis
	prompt := fmt.Sprintf(
		"Generate a Dockerfile for a %s application using %s framework. "+
			"Dependencies: %v, Entry point: %s, Port: %d, Build command: %s, Run command: %s",
		analysis.Language, analysis.Framework, analysis.Dependencies,
		analysis.EntryPoint, analysis.Port, analysis.BuildCommand, analysis.RunCommand,
	)

	return p.ai.Analyze(ctx, prompt)
}
