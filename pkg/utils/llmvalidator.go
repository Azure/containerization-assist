package llmvalidator

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/logger"
)

type LLMConfig struct {
	Endpoint       string
	APIKey         string
	DeploymentID   string
	AzOpenAIClient *ai.AzOpenAIClient
}

func ValidateLLM(ctx context.Context, llmConfig LLMConfig) error {

	_, err := url.ParseRequestURI(llmConfig.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if llmConfig.APIKey == "" {
		return errors.New("API key is missing")
	}

	if llmConfig.DeploymentID == "" {
		return errors.New("deployment ID is missing")
	}

	promptText := "This is a test prompt to validate the connection. Please respond with a simple confirmation message."

	content, tokenUsage, err := llmConfig.AzOpenAIClient.GetChatCompletion(ctx, promptText)
	if err != nil {
		logger.Errorf("failed to get chat completion: %v", err)
		return fmt.Errorf("LLM validation failed: failed to get chat completion using AzOpenAIClient: %v", err)
	}

	if content == "" {
		logger.Errorf("LLM validation failed: empty response content")
		return errors.New("LLM validation failed: empty response content")
	}

	logger.Infof("LLM validation successful: %s", content)
	logger.Debugf("Token usage: Total tokens: %d, Prompt tokens: %d, Completion tokens: %d", tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)

	return nil
}
