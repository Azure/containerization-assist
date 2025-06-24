package analyzer

import (
	"context"
)

// TokenUsage holds the token usage information
type TokenUsage struct {
	CompletionTokens int
	PromptTokens     int
	TotalTokens      int
}

// Analyzer provides a unified interface for all analysis operations
// This interface can be implemented by CallerAnalyzer (MCP) or StubAnalyzer
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
