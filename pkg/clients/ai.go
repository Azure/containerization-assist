package clients

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/logger"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

func (c *Clients) TestOpenAIConn(ctx context.Context) error {
	content, tokenUsage, err := c.AzOpenAIClient.GetChatCompletion(ctx, "Hello Azure OpenAI! Tell me this is working in one short sentence.")
	if err != nil {
		return errors.New(errors.CodeNetworkError, "ai", fmt.Sprintf("failed to get chat completion: %v", err), err)
	}

	logger.Info("Azure OpenAI Test")
	logger.Infof("Response: %s", content)
	logger.Infof("Total tokens used: %d, Prompt tokens: %d, Completion tokens: %d", tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
	return nil
}
