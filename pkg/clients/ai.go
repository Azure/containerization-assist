package clients

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/logger"
)

func (c *Clients) TestOpenAIConn(ctx context.Context) error {
	content, tokenUsage, err := c.AzOpenAIClient.GetChatCompletion(ctx, "Hello Azure OpenAI! Tell me this is working in one short sentence.")
	if err != nil {
		return fmt.Errorf("failed to get chat completion: %w", err)
	}

	logger.Info("Azure OpenAI Test")
	logger.Infof("Response: %s", content)
	logger.Infof("Total tokens used: %d, Prompt tokens: %d, Completion tokens: %d", tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
	return nil
}
