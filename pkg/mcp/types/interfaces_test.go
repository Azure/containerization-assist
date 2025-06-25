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

// MockProgressReporter implements ProgressReporter interface
type MockProgressReporter struct{}

func (m *MockProgressReporter) ReportStage(stageProgress float64, message string) {}

func (m *MockProgressReporter) NextStage(message string) {}

func (m *MockProgressReporter) SetStage(stageIndex int, message string) {}

func (m *MockProgressReporter) ReportOverall(progress float64, message string) {}

func (m *MockProgressReporter) GetCurrentStage() (int, ProgressStage) {
	return 0, ProgressStage{
		Name:        "test",
		Weight:      0.5,
		Description: "test stage",
	}
}

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

	// Test ProgressReporter interface
	var progressReporter ProgressReporter = &MockProgressReporter{}
	idx, stage := progressReporter.GetCurrentStage()
	if idx != 0 || stage.Name != "test" {
		t.Error("ProgressReporter interface not properly implemented")
	}

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

	// Test enum values
	if CircuitBreakerOpen == "" || CircuitBreakerClosed == "" {
		t.Error("CircuitBreaker constants not defined")
	}

	t.Log("All interface types are properly defined")
}
