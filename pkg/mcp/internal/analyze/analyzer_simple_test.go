package analyze

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Test CreateAnalyzerFromEnv function
func TestCreateAnalyzerFromEnv_Simple(t *testing.T) {
	logger := zerolog.Nop()

	analyzer := CreateAnalyzerFromEnv(logger)
	if analyzer == nil {
		t.Error("CreateAnalyzerFromEnv should not return nil")
	}

	// Test that we can call basic methods on the analyzer
	ctx := context.Background()
	result, err := analyzer.Analyze(ctx, "test prompt")
	// Stub analyzer returns specific error messages
	if err == nil {
		t.Error("Stub analyzer should return error for AI analysis")
	}
	if result != "" {
		t.Error("Stub analyzer should return empty result on error")
	}
}

// Test AnalyzerFactory with minimal configuration
func TestAnalyzerFactory_Simple(t *testing.T) {
	logger := zerolog.Nop()

	// Test with AI disabled (simplest case)
	factory := NewAnalyzerFactory(logger, false, nil)
	if factory == nil {
		t.Error("NewAnalyzerFactory should not return nil")
	}

	analyzer := factory.CreateAnalyzer()
	if analyzer == nil {
		t.Error("CreateAnalyzer should not return nil")
	}

	// Test setting analyzer options
	opts := CallerAnalyzerOpts{
		ToolName:       "test-tool",
		SystemPrompt:   "Test system prompt",
		PerCallTimeout: 30 * time.Second,
	}

	factory.SetAnalyzerOptions(opts)

	// Should still be able to create analyzer after setting options
	analyzer2 := factory.CreateAnalyzer()
	if analyzer2 == nil {
		t.Error("CreateAnalyzer should work after setting options")
	}
}

// Test AnalyzerConfig structure and methods
func TestAnalyzerConfig_Simple(t *testing.T) {
	config := DefaultAnalyzerConfig()
	if config == nil {
		t.Error("DefaultAnalyzerConfig should not return nil")
		return
	}

	// Test default values
	if config.EnableAI {
		t.Error("Default config should have AI disabled")
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be 'info', got '%s'", config.LogLevel)
	}
	if config.MaxPromptLength != 4096 {
		t.Errorf("Expected MaxPromptLength to be 4096, got %d", config.MaxPromptLength)
	}
	if !config.CacheEnabled {
		t.Error("Expected CacheEnabled to be true")
	}
	if config.CacheTTLSeconds != 300 {
		t.Errorf("Expected CacheTTLSeconds to be 300, got %d", config.CacheTTLSeconds)
	}

	// Test LoadFromEnv doesn't panic
	config.LoadFromEnv()
}

// Test CallerAnalyzerOpts structure
func TestCallerAnalyzerOpts_Structure(t *testing.T) {
	opts := CallerAnalyzerOpts{
		ToolName:       "custom-tool",
		SystemPrompt:   "Custom system prompt for testing",
		PerCallTimeout: 45 * time.Second,
	}

	if opts.ToolName != "custom-tool" {
		t.Errorf("Expected ToolName to be 'custom-tool', got '%s'", opts.ToolName)
	}
	if opts.SystemPrompt != "Custom system prompt for testing" {
		t.Errorf("Expected SystemPrompt to be 'Custom system prompt for testing', got '%s'", opts.SystemPrompt)
	}
	if opts.PerCallTimeout != 45*time.Second {
		t.Errorf("Expected PerCallTimeout to be 45s, got %v", opts.PerCallTimeout)
	}
}

// Test CreateAnalyzerFromConfig
func TestCreateAnalyzerFromConfig_Simple(t *testing.T) {
	logger := zerolog.Nop()

	// Test with AI disabled
	config := &AnalyzerConfig{
		EnableAI:        false,
		LogLevel:        "debug",
		MaxPromptLength: 2048,
		CacheEnabled:    false,
		CacheTTLSeconds: 600,
	}

	analyzer := CreateAnalyzerFromConfig(config, logger)
	if analyzer == nil {
		t.Error("CreateAnalyzerFromConfig should not return nil")
	}

	// Test with AI enabled (will still return stub without transport)
	config.EnableAI = true
	analyzer2 := CreateAnalyzerFromConfig(config, logger)
	if analyzer2 == nil {
		t.Error("CreateAnalyzerFromConfig should not return nil even with AI enabled")
	}
}

// Test StubAnalyzer functionality
func TestStubAnalyzer_Basic(t *testing.T) {
	analyzer := NewStubAnalyzer()
	if analyzer == nil {
		t.Error("NewStubAnalyzer should not return nil")
	}

	ctx := context.Background()

	// Test Analyze method - stub analyzer returns errors
	result, err := analyzer.Analyze(ctx, "test prompt")
	if err == nil {
		t.Error("StubAnalyzer.Analyze should return error indicating AI not available")
	}
	if result != "" {
		t.Error("StubAnalyzer.Analyze should return empty result on error")
	}

	// Test AnalyzeWithFileTools method - stub analyzer returns errors
	result2, err := analyzer.AnalyzeWithFileTools(ctx, "test prompt", "/test/dir")
	if err == nil {
		t.Error("StubAnalyzer.AnalyzeWithFileTools should return error indicating AI not available")
	}
	if result2 != "" {
		t.Error("StubAnalyzer.AnalyzeWithFileTools should return empty result on error")
	}

	// Test AnalyzeWithFormat method - stub analyzer returns errors
	result3, err := analyzer.AnalyzeWithFormat(ctx, "test prompt", "json")
	if err == nil {
		t.Error("StubAnalyzer.AnalyzeWithFormat should return error indicating AI not available")
	}
	if result3 != "" {
		t.Error("StubAnalyzer.AnalyzeWithFormat should return empty result on error")
	}

	// Test token usage methods
	usage := analyzer.GetTokenUsage()
	// TokenUsage is a struct, so we just check if it has reasonable values
	if usage.PromptTokens < 0 {
		t.Error("PromptTokens should not be negative")
	}

	analyzer.ResetTokenUsage()
	// Should not panic
}

// Test that analyzer factory handles AI enabled with transport
func TestAnalyzerFactory_WithTransport(t *testing.T) {
	logger := zerolog.Nop()

	// Create a mock transport
	mockTransport := &MockTransport{
		response: "Mock analysis result",
	}

	factory := NewAnalyzerFactory(logger, true, mockTransport)
	if factory == nil {
		t.Error("NewAnalyzerFactory should not return nil")
	}

	analyzer := factory.CreateAnalyzer()
	if analyzer == nil {
		t.Error("CreateAnalyzer should not return nil")
	}

	// Test that we can analyze with the mock transport
	ctx := context.Background()
	result, err := analyzer.Analyze(ctx, "test prompt")
	if err != nil {
		t.Errorf("Analyze should not return error with mock transport, got: %v", err)
	}
	if result != "Mock analysis result" {
		t.Errorf("Expected result to be 'Mock analysis result', got '%s'", result)
	}
}

// MockTransport for testing
type MockTransport struct {
	response string
	err      error
}

func (m *MockTransport) SendPrompt(prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}
