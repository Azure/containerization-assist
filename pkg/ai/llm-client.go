package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

type AzOpenAIClient struct {
	client                *azopenai.Client
	deploymentID          string
	dockerfileChatHistory []azopenai.ChatRequestMessageClassification
	manifestChatHistory   []azopenai.ChatRequestMessageClassification
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
		manifestChatHistory:   make([]azopenai.ChatRequestMessageClassification, 0),
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

// printChatHistory formats any chat history with a heading
func printChatHistory(history []azopenai.ChatRequestMessageClassification, heading string) string {
	if len(history) == 0 {
		return fmt.Sprintf("No %s available.", strings.ToLower(heading))
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("\n=== %s ===\n\n", heading))

	for i, message := range history {
		result.WriteString(fmt.Sprintf("--- Message %d ---\n", i+1))

		switch msg := message.(type) {
		case *azopenai.ChatRequestUserMessage:
			result.WriteString("Role: User\n")
			contentStr := fmt.Sprintf("%v", msg.Content)
			result.WriteString(fmt.Sprintf("Content: %s\n", contentStr))
		case *azopenai.ChatRequestAssistantMessage:
			result.WriteString("Role: Assistant\n")
			if content := msg.Content; content != nil {
				result.WriteString(fmt.Sprintf("Content: %s\n", *content))
			}
		case *azopenai.ChatRequestSystemMessage:
			result.WriteString("Role: System\n")
			result.WriteString(fmt.Sprintf("Content: %s\n", msg.Content))
		default:
			result.WriteString(fmt.Sprintf("Role: Unknown (Type: %T)\n", msg))
		}

		result.WriteString("\n")
	}

	return result.String()
}

// PrintDockerfileChatHistory formats and prints the Dockerfile chat history
func (c *AzOpenAIClient) PrintDockerfileChatHistory() string {
	return printChatHistory(c.dockerfileChatHistory, "Dockerfile Chat History")
}

// AddToManifestChatHistory adds a message to the manifest chat history
func (c *AzOpenAIClient) AddToManifestChatHistory(message azopenai.ChatRequestMessageClassification) {
	c.manifestChatHistory = append(c.manifestChatHistory, message)
}

// GetManifestChatHistory returns the current manifest chat history
func (c *AzOpenAIClient) GetManifestChatHistory() []azopenai.ChatRequestMessageClassification {
	return c.manifestChatHistory
}

// GetManifestChatCompletion sends a prompt related to Kubernetes manifest generation with history context
func (c *AzOpenAIClient) GetManifestChatCompletion(promptText string) (string, error) {
	return c.GetChatCompletionWithMemory(promptText, c.manifestChatHistory)
}

// PrintManifestChatHistory formats and prints the Kubernetes manifest chat history
func (c *AzOpenAIClient) PrintManifestChatHistory() string {
	return printChatHistory(c.manifestChatHistory, "Kubernetes Manifest Chat History")
}
