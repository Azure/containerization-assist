package ai

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/container-copilot/pkg/logger"
)

type AzOpenAIClient struct {
	client       *azopenai.Client
	deploymentID string
}

// NewAzOpenAIClient creates and returns a new AzOpenAIClient using the provided credentials
// The deploymentID is stored and used for all subsequent API calls
func NewAzOpenAIClient(endpoint, apiKey, deploymentID string) (*AzOpenAIClient, error) {
	keyCredential := azcore.NewKeyCredential(apiKey)
	client, err := azopenai.NewClientWithKeyCredential(endpoint, keyCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating Azure OpenAI client: %v", err)
	}
	return &AzOpenAIClient{
		client:       client,
		deploymentID: deploymentID,
	}, nil
}

// GetChatCompletion sends a prompt to the LLM and returns the completion text.
func (c *AzOpenAIClient) GetChatCompletion(ctx context.Context, promptText string) (string, error) {
	approxTokens := len(promptText) / 4
	logger.Debugf("Calling GetChatCompletion with approxTokens: %d", approxTokens)
	resp, err := c.client.GetChatCompletions(
		ctx,
		azopenai.ChatCompletionsOptions{
			DeploymentName: to.Ptr(c.deploymentID),
			Messages: []azopenai.ChatRequestMessageClassification{
				&azopenai.ChatRequestUserMessage{
					Content: azopenai.NewChatRequestUserMessageContent(promptText),
				},
			},
		},
		nil,
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
		return *resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no completion received from LLM")
}

// Does a GetChatCompletion but fills the promptText in %s
func (c *AzOpenAIClient) GetChatCompletionWithFormat(ctx context.Context, promptText string, args ...interface{}) (string, error) {
	promptText = fmt.Sprintf(promptText, args...)
	return c.GetChatCompletion(ctx, promptText)
}
