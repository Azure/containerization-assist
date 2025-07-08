package core

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// MockAIAnalyzer is a mock implementation for testing
type MockAIAnalyzer struct{}

func (m *MockAIAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "mock analysis", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "mock analysis with files", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "mock formatted analysis", nil
}

func (m *MockAIAnalyzer) GetTokenUsage() types.TokenUsage {
	return types.TokenUsage{}
}

func (m *MockAIAnalyzer) ResetTokenUsage() {}

// AnalysisService interface methods
func (m *MockAIAnalyzer) AnalyzeRepository(ctx context.Context, path string, callback ProgressCallback) (*RepositoryAnalysis, error) {
	if callback != nil {
		callback("mock analyzing", 50, 100)
		callback("mock completed", 100, 100)
	}
	return &RepositoryAnalysis{
		Language:     "go",
		Framework:    "test",
		Dependencies: []string{"test-dep"},
		Structure:    map[string]interface{}{"test": true},
		Metrics:      map[string]float64{"coverage": 80.0},
		Issues:       []AnalysisIssue{},
		Suggestions:  []string{"Mock suggestion"},
	}, nil
}

func (m *MockAIAnalyzer) AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error) {
	return &AIAnalysis{
		Summary:         "Mock AI analysis",
		Recommendations: []string{"Test recommendation"},
		Confidence:      0.9,
		Analysis:        map[string]interface{}{"mock": true},
		Metadata:        map[string]interface{}{"test": true},
	}, nil
}

func (m *MockAIAnalyzer) GetAnalysisProgress(ctx context.Context, analysisID string) (*AnalysisProgress, error) {
	return &AnalysisProgress{
		ID:       analysisID,
		Stage:    "completed",
		Progress: 100,
		Total:    100,
		Complete: true,
		Messages: []string{"Mock analysis completed"},
	}, nil
}

// Test SetAnalyzer and ValidateAnalyzerForProduction (avoiding constructor tests due to complex dependencies)
func TestMCPClientsAnalyzerOperations(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.Disabled)

	// Create a minimal MCPClients struct for testing analyzer operations
	clients := &MCPClients{}

	// Test direct analyzer assignment (replaces SetAnalyzer)
	analyzer := &MockAIAnalyzer{}
	clients.Analyzer = analyzer

	if clients.Analyzer != analyzer {
		t.Error("Analyzer not set correctly")
	}
	if _, ok := clients.Analyzer.(*MockAIAnalyzer); !ok {
		t.Error("Analyzer should be MockAIAnalyzer type")
	}

	// Test ValidateAnalyzerForProduction with nil analyzer (should fail)
	clients.Analyzer = nil
	err := clients.ValidateAnalyzerForProduction(logger)
	if err == nil {
		t.Error("Nil analyzer should fail validation")
	}
	// Check if it's a RichError and extract the message
	if richErr, ok := err.(*errors.RichError); ok {
		if richErr.Message != "analyzer cannot be nil" {
			t.Errorf("Expected 'analyzer cannot be nil', got %s", richErr.Message)
		}
	} else if err.Error() != "analyzer cannot be nil" {
		t.Errorf("Expected 'analyzer cannot be nil', got %s", err.Error())
	}

	// Test with unknown analyzer type (should fail)
	clients.Analyzer = (&MockAIAnalyzer{})
	err = clients.ValidateAnalyzerForProduction(logger)
	if err == nil {
		t.Error("Unknown analyzer type should fail validation")
	}
	if !strings.Contains(err.Error(), "may not be safe for production") {
		t.Errorf("Error should mention production safety: %s", err.Error())
	}

	// Test with stub analyzer (should pass)
	clients.Analyzer = (&stubAnalyzer{})
	err = clients.ValidateAnalyzerForProduction(logger)
	if err != nil {
		t.Errorf("Stub analyzer should be valid for production: %v", err)
	}
}

// Test stubAnalyzer implementation
func TestStubAnalyzer(t *testing.T) {
	stub := &stubAnalyzer{}

	// Test Analyze
	result, err := stub.Analyze(context.Background(), "test prompt")
	if err != nil {
		t.Errorf("Stub analyzer should not return error: %v", err)
	}
	if result != "stub analysis result" {
		t.Errorf("Expected 'stub analysis result', got %s", result)
	}

	// Test AnalyzeWithFileTools
	result, err = stub.AnalyzeWithFileTools(context.Background(), "test", "/tmp")
	if err != nil {
		t.Errorf("Stub analyzer should not return error: %v", err)
	}
	if result != "stub analysis result" {
		t.Errorf("Expected 'stub analysis result', got %s", result)
	}

	// Test AnalyzeWithFormat
	result, err = stub.AnalyzeWithFormat(context.Background(), "template %s", "arg")
	if err != nil {
		t.Errorf("Stub analyzer should not return error: %v", err)
	}
	if result != "stub analysis result" {
		t.Errorf("Expected 'stub analysis result', got %s", result)
	}

	// Test GetTokenUsage
	usage := stub.GetTokenUsage()
	if usage.TotalTokens != 0 || usage.PromptTokens != 0 || usage.CompletionTokens != 0 {
		t.Error("Stub analyzer should return empty token usage")
	}

	// Test ResetTokenUsage (should not panic)
	stub.ResetTokenUsage() // Should complete without error
}

// Test stubAnalyzer interface conformance
func TestStubAnalyzerInterface(t *testing.T) {
	var analyzer AnalysisService = &stubAnalyzer{}

	// Verify it implements the AnalysisService interface correctly
	result, err := analyzer.AnalyzeRepository(context.Background(), "/tmp", nil)
	if err != nil {
		t.Errorf("stubAnalyzer should not return error: %v", err)
	}
	if result == nil {
		t.Error("stubAnalyzer should return analysis result")
	}

	aiResult, err := analyzer.AnalyzeWithAI(context.Background(), "test content")
	if err != nil {
		t.Errorf("stubAnalyzer should not return error: %v", err)
	}
	if aiResult == nil {
		t.Error("stubAnalyzer should return AI analysis result")
	}

	progress, err := analyzer.GetAnalysisProgress(context.Background(), "test-id")
	if err != nil {
		t.Errorf("stubAnalyzer should not return error: %v", err)
	}
	if progress == nil {
		t.Error("stubAnalyzer should return analysis progress")
	}
}

// Test MockAIAnalyzer interface conformance
func TestMockAnalyzerInterface(t *testing.T) {
	var analyzer AnalysisService = &MockAIAnalyzer{}

	// Test AnalyzeRepository
	result, err := analyzer.AnalyzeRepository(context.Background(), "/tmp", nil)
	if err != nil {
		t.Errorf("MockAIAnalyzer should not return error: %v", err)
	}
	if result == nil || result.Language != "go" {
		t.Error("MockAIAnalyzer should return valid analysis result")
	}

	// Test AnalyzeWithAI
	aiResult, err := analyzer.AnalyzeWithAI(context.Background(), "test content")
	if err != nil {
		t.Errorf("MockAIAnalyzer should not return error: %v", err)
	}
	if aiResult == nil || aiResult.Summary != "Mock AI analysis" {
		t.Error("MockAIAnalyzer should return valid AI analysis result")
	}

	// Test GetAnalysisProgress
	progress, err := analyzer.GetAnalysisProgress(context.Background(), "test-id")
	if err != nil {
		t.Errorf("MockAIAnalyzer should not return error: %v", err)
	}
	if progress == nil || progress.Stage != "completed" {
		t.Error("MockAIAnalyzer should return valid analysis progress")
	}
}

// Test MCPClients struct creation and field access
func TestMCPClientsStructure(t *testing.T) {
	// Test MCPClients struct can be created
	clients := &MCPClients{}

	// Test that analyzer can be set and accessed
	analyzer := &stubAnalyzer{}
	clients.Analyzer = analyzer

	if clients.Analyzer == nil {
		t.Error("Analyzer field should not be nil after assignment")
	}
	if _, ok := clients.Analyzer.(*stubAnalyzer); !ok {
		t.Error("Analyzer should be stubAnalyzer type")
	}

	// Test that fields are accessible (even if nil)
	// This tests the struct field definitions without requiring complex dependencies
	if &clients.Docker == nil {
		t.Error("Docker field should be accessible")
	}
	if &clients.Kind == nil {
		t.Error("Kind field should be accessible")
	}
	if &clients.Kube == nil {
		t.Error("Kube field should be accessible")
	}
}
