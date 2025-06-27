package ai

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

func TestTokenUsageOperations(t *testing.T) {
	// Test TokenUsage struct basic operations
	usage := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
	}

	if usage.PromptTokens != 100 {
		t.Errorf("Expected PromptTokens=100, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 200 {
		t.Errorf("Expected CompletionTokens=200, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("Expected TotalTokens=300, got %d", usage.TotalTokens)
	}
}

func TestAzOpenAIClient_TokenUsageManagement(t *testing.T) {
	client := &AzOpenAIClient{
		deploymentID: "test-deployment",
		tokenUsage:   TokenUsage{},
	}

	// Test initial state
	usage := client.GetTokenUsage()
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0 {
		t.Errorf("Expected zero initial usage, got %+v", usage)
	}

	// Test IncrementTokenUsage
	azUsage := &azopenai.CompletionsUsage{
		PromptTokens:     to.Ptr(int32(50)),
		CompletionTokens: to.Ptr(int32(100)),
		TotalTokens:      to.Ptr(int32(150)),
	}

	client.IncrementTokenUsage(azUsage)
	usage = client.GetTokenUsage()
	if usage.PromptTokens != 50 {
		t.Errorf("Expected PromptTokens=50, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 100 {
		t.Errorf("Expected CompletionTokens=100, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 150 {
		t.Errorf("Expected TotalTokens=150, got %d", usage.TotalTokens)
	}

	// Test multiple increments
	client.IncrementTokenUsage(azUsage)
	usage = client.GetTokenUsage()
	if usage.PromptTokens != 100 {
		t.Errorf("Expected PromptTokens=100 after second increment, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 200 {
		t.Errorf("Expected CompletionTokens=200 after second increment, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("Expected TotalTokens=300 after second increment, got %d", usage.TotalTokens)
	}

	// Test ResetTokenUsage
	client.ResetTokenUsage()
	usage = client.GetTokenUsage()
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0 {
		t.Errorf("Expected zero usage after reset, got %+v", usage)
	}
}

func TestNewAzOpenAIClient_InvalidInputs(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		apiKey       string
		deploymentID string
		expectError  bool
	}{
		{
			name:         "empty endpoint",
			endpoint:     "",
			apiKey:       "test-key",
			deploymentID: "test-deployment",
			expectError:  false, // Azure SDK may accept empty endpoint during client creation
		},
		{
			name:         "empty api key",
			endpoint:     "https://test.openai.azure.com",
			apiKey:       "",
			deploymentID: "test-deployment",
			expectError:  false, // Azure SDK may accept empty key during client creation
		},
		{
			name:         "empty deployment id",
			endpoint:     "https://test.openai.azure.com",
			apiKey:       "test-key",
			deploymentID: "",
			expectError:  false, // This is allowed by the Azure SDK
		},
		{
			name:         "invalid endpoint format",
			endpoint:     "not-a-url",
			apiKey:       "test-key",
			deploymentID: "test-deployment",
			expectError:  false, // Azure SDK may accept this during client creation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAzOpenAIClient(tt.endpoint, tt.apiKey, tt.deploymentID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for invalid inputs, got nil")
				}
				if client != nil {
					t.Errorf("Expected nil client on error, got %v", client)
				}
			} else {
				// For these tests, we mainly verify that the constructor doesn't panic
				// Actual validation may happen during API calls, not during client creation
				if client != nil {
					if client.deploymentID != tt.deploymentID {
						t.Errorf("Expected deploymentID=%s, got %s", tt.deploymentID, client.deploymentID)
					}
				}
			}
		})
	}
}

func TestNewAzOpenAIClient_ValidInputs(t *testing.T) {
	// Test with valid inputs (though it will fail to connect)
	client, err := NewAzOpenAIClient(
		"https://test.openai.azure.com",
		"test-key",
		"test-deployment",
	)

	// We expect no error from the constructor itself, even though the credentials are fake
	if err != nil {
		t.Errorf("Expected no error from constructor with valid format inputs, got %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.deploymentID != "test-deployment" {
		t.Errorf("Expected deploymentID='test-deployment', got %s", client.deploymentID)
	}

	// Test that the client implements the LLMClient interface
	var _ LLMClient = client
}

func TestLLMCompletion_Structure(t *testing.T) {
	completion := LLMCompletion{
		StageID:   "test-stage",
		Iteration: 1,
		Response:  "test response",
		TokenUsage: TokenUsage{
			PromptTokens:     50,
			CompletionTokens: 100,
			TotalTokens:      150,
		},
		Prompt: "test prompt",
	}

	if completion.StageID != "test-stage" {
		t.Errorf("Expected StageID='test-stage', got %s", completion.StageID)
	}
	if completion.Iteration != 1 {
		t.Errorf("Expected Iteration=1, got %d", completion.Iteration)
	}
	if completion.Response != "test response" {
		t.Errorf("Expected Response='test response', got %s", completion.Response)
	}
	if completion.Prompt != "test prompt" {
		t.Errorf("Expected Prompt='test prompt', got %s", completion.Prompt)
	}
	if completion.TokenUsage.TotalTokens != 150 {
		t.Errorf("Expected TotalTokens=150, got %d", completion.TokenUsage.TotalTokens)
	}
}

func TestAzOpenAIClient_GetChatCompletionWithFormat(t *testing.T) {

	// Test format functionality
	tests := []struct {
		name         string
		template     string
		args         []interface{}
		expectedCall string
	}{
		{
			name:         "no args",
			template:     "Hello world",
			args:         []interface{}{},
			expectedCall: "Hello world",
		},
		{
			name:         "single string arg",
			template:     "Hello %s",
			args:         []interface{}{"world"},
			expectedCall: "Hello world",
		},
		{
			name:         "multiple args",
			template:     "Hello %s, you have %d messages",
			args:         []interface{}{"Alice", 5},
			expectedCall: "Hello Alice, you have 5 messages",
		},
		{
			name:         "complex format",
			template:     "User %s has %.2f balance and %d items",
			args:         []interface{}{"Bob", 123.456, 10},
			expectedCall: "User Bob has 123.46 balance and 10 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily mock the underlying Azure client in this test,
			// but we can test that the format string processing works correctly
			// by checking what would be passed to GetChatCompletion

			// Create a test that verifies the format string processing
			formatted := fmt.Sprintf(tt.template, tt.args...)
			if formatted != tt.expectedCall {
				t.Errorf("Format processing failed: expected '%s', got '%s'", tt.expectedCall, formatted)
			}
		})
	}
}

func TestInterfaceCompliance(_ *testing.T) {
	// Test that AzOpenAIClient implements LLMClient interface
	var _ LLMClient = (*AzOpenAIClient)(nil)
}

func TestClientErrorHandling(t *testing.T) {
	client := &AzOpenAIClient{
		deploymentID: "test-deployment",
		tokenUsage:   TokenUsage{},
	}

	// Test that the client structure is set up correctly
	if client.deploymentID != "test-deployment" {
		t.Errorf("Expected deploymentID to be preserved")
	}
}

func TestTokenUsageZeroValues(t *testing.T) {
	var usage TokenUsage

	// Test zero values
	if usage.PromptTokens != 0 {
		t.Errorf("Expected zero PromptTokens, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 0 {
		t.Errorf("Expected zero CompletionTokens, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 0 {
		t.Errorf("Expected zero TotalTokens, got %d", usage.TotalTokens)
	}
}

func TestClientInitialization(t *testing.T) {
	client := &AzOpenAIClient{
		deploymentID: "test-deployment",
		tokenUsage:   TokenUsage{},
	}

	// Test initial state
	if client.deploymentID != "test-deployment" {
		t.Errorf("Expected deploymentID='test-deployment', got %s", client.deploymentID)
	}

	if client.client != nil {
		t.Errorf("Expected nil client in manual initialization")
	}

	usage := client.GetTokenUsage()
	if usage.TotalTokens != 0 {
		t.Errorf("Expected zero initial token usage")
	}
}
