package analyze

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// PartialRepositoryAnalyzer implements full Analyzer interface but only provides repository functionality
// AI methods return not-implemented errors
type PartialRepositoryAnalyzer struct {
	repo core.RepositoryAnalyzer
}

// NewPartialRepositoryAnalyzer creates an Analyzer that only supports repository analysis
func NewPartialRepositoryAnalyzer(repo core.RepositoryAnalyzer) core.Analyzer {
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

// PartialAIAnalyzer implements full Analyzer interface but only provides AI functionality
// Repository methods return not-implemented errors
type PartialAIAnalyzer struct {
	ai core.AIAnalyzer
}

// NewPartialAIAnalyzer creates an Analyzer that only supports AI analysis
func NewPartialAIAnalyzer(ai core.AIAnalyzer) core.Analyzer {
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
