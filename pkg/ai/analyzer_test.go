package ai

import (
	"context"
	"testing"
)

// AnalysisError represents an error during analysis
type AnalysisError struct {
	Message string
}

func (e *AnalysisError) Error() string {
	return e.Message
}

// MockAnalyzer implements the Analyzer interface for testing
type MockAnalyzer struct {
	analyzeResult string
	analyzeError  error
	tokenUsage    TokenUsage
	callHistory   []string
}

func NewMockAnalyzer() *MockAnalyzer {
	return &MockAnalyzer{
		analyzeResult: "mock analysis result",
		tokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 200,
			TotalTokens:      300,
		},
		callHistory: make([]string, 0),
	}
}

func (m *MockAnalyzer) SetResult(result string, err error) {
	m.analyzeResult = result
	m.analyzeError = err
}

func (m *MockAnalyzer) Analyze(_ context.Context, prompt string) (string, error) {
	m.callHistory = append(m.callHistory, "Analyze: "+prompt)
	return m.analyzeResult, m.analyzeError
}

func (m *MockAnalyzer) AnalyzeWithFileTools(_ context.Context, prompt, baseDir string) (string, error) {
	m.callHistory = append(m.callHistory, "AnalyzeWithFileTools: "+prompt+" ("+baseDir+")")
	return m.analyzeResult, m.analyzeError
}

func (m *MockAnalyzer) AnalyzeWithFormat(_ context.Context, promptTemplate string, _ ...interface{}) (string, error) {
	m.callHistory = append(m.callHistory, "AnalyzeWithFormat: "+promptTemplate)
	return m.analyzeResult, m.analyzeError
}

func (m *MockAnalyzer) GetTokenUsage() TokenUsage {
	return m.tokenUsage
}

func (m *MockAnalyzer) ResetTokenUsage() {
	m.tokenUsage = TokenUsage{}
}

func (m *MockAnalyzer) GetCallHistory() []string {
	return m.callHistory
}

func TestMockAnalyzer_Analyze(t *testing.T) {
	analyzer := NewMockAnalyzer()
	ctx := context.Background()

	result, err := analyzer.Analyze(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "mock analysis result" {
		t.Errorf("Expected 'mock analysis result', got %s", result)
	}

	history := analyzer.GetCallHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 call in history, got %d", len(history))
	}

	if history[0] != "Analyze: test prompt" {
		t.Errorf("Expected 'Analyze: test prompt', got %s", history[0])
	}
}

func TestMockAnalyzer_AnalyzeWithFileTools(t *testing.T) {
	analyzer := NewMockAnalyzer()
	ctx := context.Background()

	result, err := analyzer.AnalyzeWithFileTools(ctx, "test prompt", "/test/dir")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "mock analysis result" {
		t.Errorf("Expected 'mock analysis result', got %s", result)
	}

	history := analyzer.GetCallHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 call in history, got %d", len(history))
	}

	expected := "AnalyzeWithFileTools: test prompt (/test/dir)"
	if history[0] != expected {
		t.Errorf("Expected '%s', got %s", expected, history[0])
	}
}

func TestMockAnalyzer_AnalyzeWithFormat(t *testing.T) {
	analyzer := NewMockAnalyzer()
	ctx := context.Background()

	result, err := analyzer.AnalyzeWithFormat(ctx, "template %s", "arg1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "mock analysis result" {
		t.Errorf("Expected 'mock analysis result', got %s", result)
	}

	history := analyzer.GetCallHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 call in history, got %d", len(history))
	}

	if history[0] != "AnalyzeWithFormat: template %s" {
		t.Errorf("Expected 'AnalyzeWithFormat: template %%s', got %s", history[0])
	}
}

func TestMockAnalyzer_TokenUsage(t *testing.T) {
	analyzer := NewMockAnalyzer()

	usage := analyzer.GetTokenUsage()
	if usage.PromptTokens != 100 {
		t.Errorf("Expected PromptTokens=100, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 200 {
		t.Errorf("Expected CompletionTokens=200, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("Expected TotalTokens=300, got %d", usage.TotalTokens)
	}

	analyzer.ResetTokenUsage()
	usage = analyzer.GetTokenUsage()
	if usage.PromptTokens != 0 {
		t.Errorf("Expected PromptTokens=0 after reset, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 0 {
		t.Errorf("Expected CompletionTokens=0 after reset, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 0 {
		t.Errorf("Expected TotalTokens=0 after reset, got %d", usage.TotalTokens)
	}
}

func TestMockAnalyzer_SetResult(t *testing.T) {
	analyzer := NewMockAnalyzer()
	ctx := context.Background()

	// Test setting a different result
	analyzer.SetResult("custom result", nil)
	result, err := analyzer.Analyze(ctx, "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "custom result" {
		t.Errorf("Expected 'custom result', got %s", result)
	}

	// Test setting an error
	testErr := &AnalysisError{Message: "test error"}
	analyzer.SetResult("", testErr)
	result, err = analyzer.Analyze(ctx, "test")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != testErr {
		t.Errorf("Expected testErr, got %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result, got %s", result)
	}
}

func TestMockAnalyzer_InterfaceCompliance(_ *testing.T) {
	var _ Analyzer = (*MockAnalyzer)(nil)
}
