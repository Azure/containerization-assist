package analyze

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/internal/config"
	"github.com/rs/zerolog"
)

func TestNewStubAnalyzer(t *testing.T) {
	analyzer := NewStubAnalyzer()

	if analyzer == nil {
		t.Fatal("NewStubAnalyzer returned nil")
	}
}

func TestStubAnalyzer_Analyze(t *testing.T) {
	analyzer := NewStubAnalyzer()
	ctx := context.Background()

	// Stub analyzer should return an error
	result, err := analyzer.Analyze(ctx, "test prompt")

	if err == nil {
		t.Error("Expected error from stub analyzer")
	}

	if result != "" {
		t.Errorf("Expected empty result, got %s", result)
	}

	if !strings.Contains(err.Error(), "stub analyzer") {
		t.Errorf("Error should mention stub analyzer, got: %v", err)
	}
}

func TestStubAnalyzer_AnalyzeWithFileTools(t *testing.T) {
	analyzer := NewStubAnalyzer()
	ctx := context.Background()

	// Stub analyzer should return an error
	result, err := analyzer.AnalyzeWithFileTools(ctx, "test prompt", "/test/dir")

	if err == nil {
		t.Error("Expected error from stub analyzer")
	}

	if result != "" {
		t.Errorf("Expected empty result, got %s", result)
	}
}

func TestStubAnalyzer_AnalyzeWithFormat(t *testing.T) {
	analyzer := NewStubAnalyzer()
	ctx := context.Background()

	// Stub analyzer should return an error
	result, err := analyzer.AnalyzeWithFormat(ctx, "test %s", "arg")

	if err == nil {
		t.Error("Expected error from stub analyzer")
	}

	if result != "" {
		t.Errorf("Expected empty result, got %s", result)
	}
}

func TestStubAnalyzer_GetTokenUsage(t *testing.T) {
	analyzer := NewStubAnalyzer()

	usage := analyzer.GetTokenUsage()

	// Should return zero usage
	if usage.TotalTokens != 0 {
		t.Errorf("Expected 0 total tokens, got %d", usage.TotalTokens)
	}

	if usage.PromptTokens != 0 {
		t.Errorf("Expected 0 prompt tokens, got %d", usage.PromptTokens)
	}

	if usage.CompletionTokens != 0 {
		t.Errorf("Expected 0 completion tokens, got %d", usage.CompletionTokens)
	}
}

func TestStubAnalyzer_ResetTokenUsage(t *testing.T) {
	analyzer := NewStubAnalyzer()

	// Should not panic
	analyzer.ResetTokenUsage()
}

func TestCreateAnalyzerFromConfig(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name     string
		config   *config.AnalyzerConfig
		wantType string
	}{
		{
			name: "AI disabled config",
			config: &config.AnalyzerConfig{
				EnableAI: false,
			},
			wantType: "StubAnalyzer",
		},
		{
			name: "AI enabled config (falls back to stub)",
			config: &config.AnalyzerConfig{
				EnableAI: true,
			},
			wantType: "StubAnalyzer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := CreateAnalyzerFromConfig(tt.config, logger)

			if analyzer == nil {
				t.Fatal("CreateAnalyzerFromConfig returned nil")
			}

			if _, ok := analyzer.(*StubAnalyzer); !ok && tt.wantType == "StubAnalyzer" {
				t.Error("CreateAnalyzerFromConfig did not return a StubAnalyzer when expected")
			}
		})
	}
}

func TestNewAnalyzerFactory(t *testing.T) {
	logger := zerolog.Nop()

	// Test without transport
	factory := NewAnalyzerFactory(logger, false, nil)

	if factory == nil {
		t.Fatal("NewAnalyzerFactory returned nil")
	}

	if factory.enableAI != false {
		t.Error("Expected AI to be disabled")
	}
}
