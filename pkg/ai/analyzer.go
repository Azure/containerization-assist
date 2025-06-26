package ai

import (
	"context"
)

// Analyzer provides a unified interface for all analysis operations
// This interface can be implemented by Azure OpenAI (CLI), CallerAnalyzer (MCP), or MockAnalyzer (tests)
type Analyzer interface {
	// Analyze performs basic text analysis with the LLM
	Analyze(ctx context.Context, prompt string) (string, error)

	// AnalyzeWithFileTools performs analysis with file system access
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)

	// AnalyzeWithFormat performs analysis with formatted prompts
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)

	// GetTokenUsage returns usage statistics (may be empty for non-Azure implementations)
	GetTokenUsage() TokenUsage

	// ResetTokenUsage resets usage statistics
	ResetTokenUsage()
}
