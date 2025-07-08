package analyze

import (
	"context"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

// UnifiedAnalyzer implements the unified core.Analyzer interface by combining
// repository analysis and AI analysis capabilities
type UnifiedAnalyzer struct {
	repoAnalyzer core.RepositoryAnalyzer
	aiAnalyzer   core.AIAnalyzer
	logger       *slog.Logger
}

// NewUnifiedAnalyzer creates a new unified analyzer that implements core.Analyzer
func NewUnifiedAnalyzer(repoAnalyzer core.RepositoryAnalyzer, aiAnalyzer core.AIAnalyzer, logger *slog.Logger) core.Analyzer {
	return &UnifiedAnalyzer{
		repoAnalyzer: repoAnalyzer,
		aiAnalyzer:   aiAnalyzer,
		logger:       logger.With("component", "unified_analyzer"),
	}
}

// Repository analysis methods (from RepositoryAnalyzer)

func (u *UnifiedAnalyzer) AnalyzeStructure(ctx context.Context, path string) (*core.RepositoryInfo, error) {
	return u.repoAnalyzer.AnalyzeStructure(ctx, path)
}

func (u *UnifiedAnalyzer) AnalyzeDockerfile(ctx context.Context, path string) (*core.DockerfileInfo, error) {
	return u.repoAnalyzer.AnalyzeDockerfile(ctx, path)
}

func (u *UnifiedAnalyzer) GetBuildRecommendations(ctx context.Context, repo *core.RepositoryInfo) (*core.BuildRecommendations, error) {
	return u.repoAnalyzer.GetBuildRecommendations(ctx, repo)
}

// AI analysis methods (from AIAnalyzer)

func (u *UnifiedAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return u.aiAnalyzer.Analyze(ctx, prompt)
}

func (u *UnifiedAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return u.aiAnalyzer.AnalyzeWithFileTools(ctx, prompt, baseDir)
}

func (u *UnifiedAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return u.aiAnalyzer.AnalyzeWithFormat(ctx, promptTemplate, args...)
}

func (u *UnifiedAnalyzer) GetTokenUsage() core.TokenUsage {
	return u.aiAnalyzer.GetTokenUsage()
}

func (u *UnifiedAnalyzer) ResetTokenUsage() {
	u.aiAnalyzer.ResetTokenUsage()
}
