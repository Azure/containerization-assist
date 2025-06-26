//go:build cli

package ai

import (
	"context"
)

// AzureAnalyzer wraps existing AzOpenAIClient for CLI compatibility
// This is only compiled when building CLI binaries
type AzureAnalyzer struct {
	client *AzOpenAIClient
}

// NewAzureAnalyzer creates an analyzer that uses Azure OpenAI
func NewAzureAnalyzer(client *AzOpenAIClient) *AzureAnalyzer {
	return &AzureAnalyzer{client: client}
}

// Analyze implements Analyzer interface
func (a *AzureAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	result, _, err := a.client.GetChatCompletion(ctx, prompt)
	return result, err
}

// AnalyzeWithFileTools implements Analyzer interface
func (a *AzureAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	result, _, err := a.client.GetChatCompletionWithFileTools(ctx, prompt, baseDir)
	return result, err
}

// AnalyzeWithFormat implements Analyzer interface
func (a *AzureAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	result, _, err := a.client.GetChatCompletionWithFormat(ctx, promptTemplate, args...)
	return result, err
}

// GetTokenUsage implements Analyzer interface
func (a *AzureAnalyzer) GetTokenUsage() TokenUsage {
	return a.client.GetTokenUsage()
}

// ResetTokenUsage implements Analyzer interface
func (a *AzureAnalyzer) ResetTokenUsage() {
	a.client.ResetTokenUsage()
}
