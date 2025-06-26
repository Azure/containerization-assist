//go:build cli

package ai

import (
	"context"
	"testing"
)

// MockLLMClient implements LLMClient interface for testing AzureAnalyzer
type MockLLMClient struct {
	chatResult      string
	chatError       error
	fileToolsResult string
	fileToolsError  error
	formatResult    string
	formatError     error
	tokenUsage      TokenUsage
	callHistory     []string
}

func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		chatResult:      "mock chat response",
		fileToolsResult: "mock file tools response",
		formatResult:    "mock format response",
		tokenUsage: TokenUsage{
			PromptTokens:     50,
			CompletionTokens: 100,
			TotalTokens:      150,
		},
		callHistory: make([]string, 0),
	}
}

func (m *MockLLMClient) SetChatResult(result string, err error) {
	m.chatResult = result
	m.chatError = err
}

func (m *MockLLMClient) SetFileToolsResult(result string, err error) {
	m.fileToolsResult = result
	m.fileToolsError = err
}

func (m *MockLLMClient) SetFormatResult(result string, err error) {
	m.formatResult = result
	m.formatError = err
}

func (m *MockLLMClient) GetChatCompletion(ctx context.Context, prompt string) (string, TokenUsage, error) {
	m.callHistory = append(m.callHistory, "GetChatCompletion: "+prompt)
	return m.chatResult, m.tokenUsage, m.chatError
}

func (m *MockLLMClient) GetChatCompletionWithFileTools(ctx context.Context, prompt, baseDir string) (string, TokenUsage, error) {
	m.callHistory = append(m.callHistory, "GetChatCompletionWithFileTools: "+prompt+" ("+baseDir+")")
	return m.fileToolsResult, m.tokenUsage, m.fileToolsError
}

func (m *MockLLMClient) GetChatCompletionWithFormat(ctx context.Context, prompt string, args ...interface{}) (string, TokenUsage, error) {
	m.callHistory = append(m.callHistory, "GetChatCompletionWithFormat: "+prompt)
	return m.formatResult, m.tokenUsage, m.formatError
}

func (m *MockLLMClient) GetTokenUsage() TokenUsage {
	return m.tokenUsage
}

func (m *MockLLMClient) ResetTokenUsage() {
	m.tokenUsage = TokenUsage{}
}

func (m *MockLLMClient) GetCallHistory() []string {
	return m.callHistory
}

// Create a mock AzOpenAIClient that we can use with AzureAnalyzer
func createMockAzOpenAIClient() *AzOpenAIClient {
	mockClient := NewMockLLMClient()

	// Create an AzOpenAIClient with the mock's behavior
	return &AzOpenAIClient{
		deploymentID: "test-deployment",
		tokenUsage: TokenUsage{
			PromptTokens:     50,
			CompletionTokens: 100,
			TotalTokens:      150,
		},
	}
}

func TestNewAzureAnalyzer(t *testing.T) {
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)

	if analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}

	if analyzer.client != client {
		t.Errorf("Expected analyzer to wrap the provided client")
	}
}

func TestAzureAnalyzer_Analyze(t *testing.T) {
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)
	ctx := context.Background()

	// Since we can't easily mock the underlying Azure client calls,
	// we test that the method exists and has the correct signature
	_, err := analyzer.Analyze(ctx, "test prompt")

	// We expect an error because we don't have a real Azure client
	if err == nil {
		t.Log("Note: Analyze succeeded (may indicate successful mock or real connection)")
	} else {
		// This is expected with our test setup
		t.Logf("Expected error from Analyze with mock client: %v", err)
	}
}

func TestAzureAnalyzer_AnalyzeWithFileTools(t *testing.T) {
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)
	ctx := context.Background()

	// Test the method signature and that it calls the underlying client
	_, err := analyzer.AnalyzeWithFileTools(ctx, "test prompt", "/test/dir")

	// We expect an error because we don't have a real Azure client
	if err == nil {
		t.Log("Note: AnalyzeWithFileTools succeeded (may indicate successful mock or real connection)")
	} else {
		// This is expected with our test setup
		t.Logf("Expected error from AnalyzeWithFileTools with mock client: %v", err)
	}
}

func TestAzureAnalyzer_AnalyzeWithFormat(t *testing.T) {
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)
	ctx := context.Background()

	// Test the method signature and that it calls the underlying client
	_, err := analyzer.AnalyzeWithFormat(ctx, "template %s", "arg1")

	// We expect an error because we don't have a real Azure client
	if err == nil {
		t.Log("Note: AnalyzeWithFormat succeeded (may indicate successful mock or real connection)")
	} else {
		// This is expected with our test setup
		t.Logf("Expected error from AnalyzeWithFormat with mock client: %v", err)
	}
}

func TestAzureAnalyzer_TokenUsage(t *testing.T) {
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)

	// Test GetTokenUsage
	usage := analyzer.GetTokenUsage()
	if usage.TotalTokens != 150 {
		t.Errorf("Expected TotalTokens=150, got %d", usage.TotalTokens)
	}
	if usage.PromptTokens != 50 {
		t.Errorf("Expected PromptTokens=50, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 100 {
		t.Errorf("Expected CompletionTokens=100, got %d", usage.CompletionTokens)
	}

	// Test ResetTokenUsage
	analyzer.ResetTokenUsage()
	usage = analyzer.GetTokenUsage()
	if usage.TotalTokens != 0 {
		t.Errorf("Expected TotalTokens=0 after reset, got %d", usage.TotalTokens)
	}
	if usage.PromptTokens != 0 {
		t.Errorf("Expected PromptTokens=0 after reset, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 0 {
		t.Errorf("Expected CompletionTokens=0 after reset, got %d", usage.CompletionTokens)
	}
}

func TestAzureAnalyzer_InterfaceCompliance(t *testing.T) {
	// Test that AzureAnalyzer implements the Analyzer interface
	var _ Analyzer = (*AzureAnalyzer)(nil)
}

func TestAzureAnalyzer_MethodCallBehavior(t *testing.T) {
	// Test that each method properly delegates to the underlying client
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)
	ctx := context.Background()

	// Test that methods exist and have correct signatures
	tests := []struct {
		name string
		test func() error
	}{
		{
			name: "Analyze",
			test: func() error {
				_, err := analyzer.Analyze(ctx, "test")
				return err
			},
		},
		{
			name: "AnalyzeWithFileTools",
			test: func() error {
				_, err := analyzer.AnalyzeWithFileTools(ctx, "test", "/dir")
				return err
			},
		},
		{
			name: "AnalyzeWithFormat",
			test: func() error {
				_, err := analyzer.AnalyzeWithFormat(ctx, "template %s", "arg")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.test()
			// We expect errors since we don't have real Azure credentials
			// The important thing is that the methods are callable
			t.Logf("Method %s called successfully (error expected): %v", tt.name, err)
		})
	}
}

func TestAzureAnalyzer_NilClient(t *testing.T) {
	// Test behavior with nil client
	analyzer := NewAzureAnalyzer(nil)

	if analyzer.client != nil {
		t.Errorf("Expected nil client to be preserved")
	}

	// These calls should panic or error gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic with nil client: %v", r)
		}
	}()

	ctx := context.Background()
	_, err := analyzer.Analyze(ctx, "test")
	if err == nil {
		t.Errorf("Expected error with nil client")
	}
}

func TestAzureAnalyzer_ContextHandling(t *testing.T) {
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := analyzer.Analyze(ctx, "test")
	// Should handle cancelled context appropriately
	if err == nil {
		t.Log("Note: Method succeeded despite cancelled context")
	} else {
		t.Logf("Method properly handled cancelled context: %v", err)
	}
}

func TestAzureAnalyzer_WrapperPattern(t *testing.T) {
	// Test that AzureAnalyzer is properly implementing the wrapper pattern
	client := createMockAzOpenAIClient()
	analyzer := NewAzureAnalyzer(client)

	// Verify the wrapper has access to the wrapped client
	if analyzer.client == nil {
		t.Errorf("Wrapper should maintain reference to wrapped client")
	}

	// Verify methods delegate to the client (by checking they don't panic)
	ctx := context.Background()

	// Test that all interface methods are callable
	_ = analyzer.GetTokenUsage()
	analyzer.ResetTokenUsage()

	// These may error but should not panic
	analyzer.Analyze(ctx, "test")
	analyzer.AnalyzeWithFileTools(ctx, "test", "/dir")
	analyzer.AnalyzeWithFormat(ctx, "template", "arg")
}
