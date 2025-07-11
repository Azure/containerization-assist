package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/ai"
)

func TestTestOpenAIConn_Success(t *testing.T) {
	mockClient := &MockLLMClient{
		result: "Hello! This is working perfectly.",
		tokenUsage: ai.TokenUsage{
			PromptTokens:     15,
			CompletionTokens: 10,
			TotalTokens:      25,
		},
		err: nil,
	}

	clients := &Clients{
		AzOpenAIClient: mockClient,
	}

	ctx := context.Background()
	err := clients.TestOpenAIConn(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestTestOpenAIConn_Error(t *testing.T) {
	mockClient := &MockLLMClient{
		result:     "",
		tokenUsage: ai.TokenUsage{},
		err:        errors.New("API connection failed"),
	}

	clients := &Clients{
		AzOpenAIClient: mockClient,
	}

	ctx := context.Background()
	err := clients.TestOpenAIConn(ctx)

	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	expectedError := "failed to get chat completion"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestTestOpenAIConn_NilClient(t *testing.T) {
	clients := &Clients{
		AzOpenAIClient: nil,
	}

	ctx := context.Background()

	// This should panic or error gracefully
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic with nil client")
		}
	}()

	_ = clients.TestOpenAIConn(ctx)
}

func TestTestOpenAIConn_EmptyResponse(t *testing.T) {
	mockClient := &MockLLMClient{
		result: "",
		tokenUsage: ai.TokenUsage{
			PromptTokens:     15,
			CompletionTokens: 0,
			TotalTokens:      15,
		},
		err: nil,
	}

	clients := &Clients{
		AzOpenAIClient: mockClient,
	}

	ctx := context.Background()
	err := clients.TestOpenAIConn(ctx)

	// Empty response should not cause an error in this implementation
	if err != nil {
		t.Errorf("Expected no error with empty response, got %v", err)
	}
}

func TestTestOpenAIConn_ContextCancellation(t *testing.T) {
	mockClient := &MockLLMClient{
		result:     "Response",
		tokenUsage: ai.TokenUsage{},
		err:        context.Canceled,
	}

	clients := &Clients{
		AzOpenAIClient: mockClient,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := clients.TestOpenAIConn(ctx)

	if err == nil {
		t.Errorf("Expected error due to cancelled context")
	}

	if !strings.Contains(err.Error(), "failed to get chat completion") {
		t.Errorf("Expected wrapped error message, got %v", err)
	}
}

func TestTestOpenAIConn_PromptContent(t *testing.T) {
	// Create a mock that captures the prompt sent to it
	var capturedPrompt string
	mockClient := &MockLLMClientWithCapture{
		result: "Test response",
		tokenUsage: ai.TokenUsage{
			PromptTokens:     20,
			CompletionTokens: 10,
			TotalTokens:      30,
		},
		err:            nil,
		capturedPrompt: &capturedPrompt,
	}

	clients := &Clients{
		AzOpenAIClient: mockClient,
	}

	ctx := context.Background()
	err := clients.TestOpenAIConn(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expectedPrompt := "Hello Azure OpenAI! Tell me this is working in one short sentence."
	if capturedPrompt != expectedPrompt {
		t.Errorf("Expected prompt '%s', got '%s'", expectedPrompt, capturedPrompt)
	}
}

// MockLLMClientWithCapture captures the prompt for testing
type MockLLMClientWithCapture struct {
	result         string
	tokenUsage     ai.TokenUsage
	err            error
	capturedPrompt *string
}

func (m *MockLLMClientWithCapture) GetChatCompletion(_ context.Context, prompt string) (string, ai.TokenUsage, error) {
	if m.capturedPrompt != nil {
		*m.capturedPrompt = prompt
	}
	return m.result, m.tokenUsage, m.err
}

func (m *MockLLMClientWithCapture) GetChatCompletionWithFileTools(_ context.Context, _, _ string) (string, ai.TokenUsage, error) {
	return m.result, m.tokenUsage, m.err
}

func (m *MockLLMClientWithCapture) GetChatCompletionWithFormat(_ context.Context, _ string, _ ...interface{}) (string, ai.TokenUsage, error) {
	return m.result, m.tokenUsage, m.err
}

func (m *MockLLMClientWithCapture) GetTokenUsage() ai.TokenUsage {
	return m.tokenUsage
}

func TestTestOpenAIConn_TokenUsageLogging(t *testing.T) {
	// Test that the function handles different token usage scenarios
	testCases := []struct {
		name       string
		tokenUsage ai.TokenUsage
	}{
		{
			name: "normal usage",
			tokenUsage: ai.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
		{
			name: "zero usage",
			tokenUsage: ai.TokenUsage{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
		},
		{
			name: "high usage",
			tokenUsage: ai.TokenUsage{
				PromptTokens:     1000,
				CompletionTokens: 2000,
				TotalTokens:      3000,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockLLMClient{
				result:     "Test response",
				tokenUsage: tc.tokenUsage,
				err:        nil,
			}

			clients := &Clients{
				AzOpenAIClient: mockClient,
			}

			ctx := context.Background()
			err := clients.TestOpenAIConn(ctx)

			if err != nil {
				t.Errorf("Expected no error for %s, got %v", tc.name, err)
			}
		})
	}
}
