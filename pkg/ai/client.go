package ai

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/common/logger"
)

func TestOpenAIConn(ctx context.Context, client LLMClient) error {
	content, tokenUsage, err := client.GetChatCompletion(ctx, "Hello Azure OpenAI! Tell me this is working in one short sentence.")
	if err != nil {
		return errors.New(errors.CodeNetworkError, "ai", fmt.Sprintf("failed to get chat completion: %v", err), err)
	}

	logger.Info("Azure OpenAI Test")
	logger.Infof("Response: %s", content)
	logger.Infof("Total tokens used: %d, Prompt tokens: %d, Completion tokens: %d", tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
	return nil
}
