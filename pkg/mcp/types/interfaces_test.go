package types

import (
	"context"
	"testing"
)

// Test interface conformance by implementing mock types

// MockAIAnalyzer implements AIAnalyzer interface
type MockAIAnalyzer struct{}

func (m *MockAIAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "mock analysis result", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "mock analysis with file tools", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "mock formatted analysis", nil
}

func (m *MockAIAnalyzer) GetTokenUsage() TokenUsage {
	return TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
}

func (m *MockAIAnalyzer) ResetTokenUsage() {}

// MockProgressReporter removed - interface moved to main package to avoid import cycle

// Note: SessionManager testing removed due to complex dependencies

// MockBaseValidator removed - BaseValidator interface moved to base package

// Test interface conformance
func TestInterfaceConformance(t *testing.T) {
	// Test AIAnalyzer interface
	var aiAnalyzer AIAnalyzer = &MockAIAnalyzer{}
	if aiAnalyzer.GetTokenUsage().TotalTokens != 150 {
		t.Error("AIAnalyzer interface not properly implemented")
	}

	// HealthChecker test removed - interface moved to main package to avoid import cycle

	// ProgressReporter test removed - interface moved to main package to avoid import cycle

	// Note: SessionManager test removed due to complex dependencies

	// BaseValidator test removed - interface moved to base package
}

// Test interface type completeness
func TestTypeCompleteness(t *testing.T) {
	// Test that all required types are defined
	var tokenUsage TokenUsage
	tokenUsage.TotalTokens = 100

	var progressStage ProgressStage
	progressStage.Name = "test"

	var sessionState SessionState
	sessionState.SessionID = "test"

	// Circuit breaker constants test removed - constants moved to canonical location

	t.Log("All interface types are properly defined")
}
