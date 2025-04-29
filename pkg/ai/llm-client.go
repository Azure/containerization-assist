package ai

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

type AzOpenAIClient struct {
	client                *azopenai.Client
	deploymentID          string
	dockerfileChatHistory []azopenai.ChatRequestMessageClassification
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
		client:                client,
		deploymentID:          deploymentID,
		dockerfileChatHistory: make([]azopenai.ChatRequestMessageClassification, 0),
	}, nil
}

// GetChatCompletionWithMemory sends a prompt to the LLM with memory of previous messages and returns the completion text.
func (c *AzOpenAIClient) GetChatCompletionWithMemory(promptText string, memory []azopenai.ChatRequestMessageClassification) (string, error) {
	messages := make([]azopenai.ChatRequestMessageClassification, 0)

	// Add memory (previous messages) if provided
	if len(memory) > 0 {
		messages = append(messages, memory...)
	}

	// Add the current prompt
	messages = append(messages, &azopenai.ChatRequestUserMessage{
		Content: azopenai.NewChatRequestUserMessageContent(promptText),
	})

	resp, err := c.client.GetChatCompletions(
		context.Background(),
		azopenai.ChatCompletionsOptions{
			DeploymentName: to.Ptr(c.deploymentID),
			Messages:       messages,
		},
		nil,
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
		// Store the response in memory if needed by calling code
		return *resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no completion received from LLM")
}

// GetChatCompletion sends a prompt to the LLM and returns the completion text.
func (c *AzOpenAIClient) GetChatCompletion(promptText string) (string, error) {
	return c.GetChatCompletionWithMemory(promptText, nil)
}

// AddToDockerfileChatHistory adds a message to the Dockerfile chat history
func (c *AzOpenAIClient) AddToDockerfileChatHistory(message azopenai.ChatRequestMessageClassification) {
	c.dockerfileChatHistory = append(c.dockerfileChatHistory, message)
}

// GetDockerfileChatHistory returns the current Dockerfile chat history
func (c *AzOpenAIClient) GetDockerfileChatHistory() []azopenai.ChatRequestMessageClassification {
	return c.dockerfileChatHistory
}

// GetDockerfileChatCompletion sends a prompt related to Dockerfile generation with history context
func (c *AzOpenAIClient) GetDockerfileChatCompletion(promptText string) (string, error) {
	return c.GetChatCompletionWithMemory(promptText, c.dockerfileChatHistory)
}
