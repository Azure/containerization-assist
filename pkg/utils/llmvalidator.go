package utils

import (
	"context"
	"errors"
	"net/url"

	"github.com/Azure/container-kit/pkg/ai"
	"github.com/Azure/container-kit/pkg/logger"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
		return mcperrors.NewError().
			Code(mcperrors.CodeValidationFailed).
			Message("Invalid endpoint URL").
			Cause(err).
			Context("endpoint", llmConfig.Endpoint).
			Context("component", "llm_validator").
			Suggestion("Provide a valid URL for the LLM endpoint").
			Build()
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
		logger.Errorf("LLM validation failed: failed to get chat completion: %v", err)
		return mcperrors.NewError().
			Code(mcperrors.NETWORK_ERROR).
			Message("Failed to get chat completion using AzOpenAIClient").
			Cause(err).
			Context("endpoint", llmConfig.Endpoint).
			Context("deployment_id", llmConfig.DeploymentID).
			Context("component", "llm_validator").
			Suggestion("Check LLM service availability and credentials").
			Build()
	}

	if content == "" {
		logger.Errorf("LLM validation failed: empty chat response content")
		return errors.New("empty chat response content")
	}

	logger.Infof("LLM validation successful: %s", content)
	logger.Debugf("Token usage: Total tokens: %d, Prompt tokens: %d, Completion tokens: %d", tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)

	return nil
}
