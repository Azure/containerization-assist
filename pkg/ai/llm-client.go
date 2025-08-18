package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/containerization-assist/pkg/common/logger"
)

type AzOpenAIClient struct {
	client       *azopenai.Client
	deploymentID string
	tokenUsage   TokenUsage
}
type LLMCompletion struct {
	StageID    string     `json:"stage_id"`
	Iteration  int        `json:"iteration"`
	Response   string     `json:"response"`
	TokenUsage TokenUsage `json:"token_usage"`
	Prompt     string     `json:"prompt"`
}

// TokenUsage holds the token usage information across all pipelines
type TokenUsage struct {
	CompletionTokens int
	PromptTokens     int
	TotalTokens      int
}
type LLMClient interface {
	GetChatCompletion(ctx context.Context, prompt string) (string, TokenUsage, error)
	GetChatCompletionWithFileTools(ctx context.Context, prompt, baseDir string) (string, TokenUsage, error)
	GetChatCompletionWithFormat(ctx context.Context, prompt string, args ...interface{}) (string, TokenUsage, error)
	GetTokenUsage() TokenUsage
}

// IncrementTokenUsage increments the client's token usage with the usage from a new API call
func (c *AzOpenAIClient) IncrementTokenUsage(usage *azopenai.CompletionsUsage) {
	c.tokenUsage.CompletionTokens += int(*usage.CompletionTokens)
	c.tokenUsage.PromptTokens += int(*usage.PromptTokens)
	c.tokenUsage.TotalTokens += int(*usage.TotalTokens)
}

// GetTokenUsage returns the current token usage statistics
func (c *AzOpenAIClient) GetTokenUsage() TokenUsage {
	return c.tokenUsage
}

// ResetTokenUsage resets the client's token usage statistics to zero
func (c *AzOpenAIClient) ResetTokenUsage() {
	c.tokenUsage = TokenUsage{}
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
func (c *AzOpenAIClient) GetChatCompletion(ctx context.Context, promptText string) (string, TokenUsage, error) {
	// Approximate the number of tokens in the input text.
	// This assumes an average token is approximately 4 characters long.
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
		return "", TokenUsage{}, err
	}

	tokenUsage := TokenUsage{
		CompletionTokens: int(*resp.Usage.CompletionTokens),
		PromptTokens:     int(*resp.Usage.PromptTokens),
		TotalTokens:      int(*resp.Usage.TotalTokens),
	}

	// Increment the client's token usage statistics
	c.IncrementTokenUsage(resp.Usage)

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
		return *resp.Choices[0].Message.Content, tokenUsage, nil
	}

	return "", tokenUsage, fmt.Errorf("no completion received from LLM")
}

// GetChatCompletionWithFileTools sends a prompt with file‐system tools (read_file, list_directory, file_exists)
// LLM is given a certaim number of turns to respond, the final assistant message is returned when the final llm reply does not contain any tool calls.
// LLM maintains the conversation history, including the tool calls and their responses.
// Returns the generated response, token usage statistics, and any error that occurred.
func (c *AzOpenAIClient) GetChatCompletionWithFileTools(
	ctx context.Context,
	prompt, baseDir string,
) (string, TokenUsage, error) {
	// 1) get our file system tools
	tools := GetFileSystemTools()

	// 2) prime the messages with the user's prompt
	messages := []azopenai.ChatRequestMessageClassification{
		&azopenai.ChatRequestUserMessage{
			Content: azopenai.NewChatRequestUserMessageContent(prompt),
		},
	}

	opts := azopenai.ChatCompletionsOptions{
		DeploymentName: to.Ptr(c.deploymentID),
		Messages:       messages,
		Tools:          tools,
		ToolChoice:     azopenai.ChatCompletionsToolChoiceAuto,
	}

	// Track the token usage for this specific function call
	thisCallUsage := TokenUsage{}

	// 3) loop, handling any tool calls, up to N turns
	for turn := range 15 {
		logger.Debugf("    tool calls turn %d", turn)
		resp, err := c.client.GetChatCompletions(ctx, opts, nil)
		if err != nil {
			return "", thisCallUsage, fmt.Errorf("chat completion failed on turn %d: %w, in GetChatCompletionWithFileTools", turn+1, err)
		}

		// Increment token usage from this API call
		if resp.Usage != nil {
			c.IncrementTokenUsage(resp.Usage) //Increments the global token usage
			thisCallUsage.CompletionTokens += int(*resp.Usage.CompletionTokens)
			thisCallUsage.PromptTokens += int(*resp.Usage.PromptTokens)
			thisCallUsage.TotalTokens += int(*resp.Usage.TotalTokens)
		}
		msg := resp.Choices[0].Message
		tcalls := msg.ToolCalls

		// Did the model invoke any tools?
		if len(tcalls) == 0 {
			// No tool calls - return the final response
			if msg.Content != nil {
				return *msg.Content, thisCallUsage, nil
			}
			return "", thisCallUsage, fmt.Errorf("empty response from LLM")
		}

		// Tools were invoked
		logger.Debugf("    invoked %d tools", len(tcalls))

		// a) echo the assistant's message with tool calls into our history
		var content *azopenai.ChatRequestAssistantMessageContent
		if msg.Content != nil {
			content = azopenai.NewChatRequestAssistantMessageContent(*msg.Content)
		}

		assistantMsg := &azopenai.ChatRequestAssistantMessage{
			Content: content,
		}

		// Add tool calls directly to the assistant message to maintain conversation history
		assistantMsg.ToolCalls = msg.ToolCalls
		messages = append(messages, assistantMsg)

		// b) execute each tool and append its response
		for _, tc := range msg.ToolCalls {
			// Only process function tool calls
			if funcTC, ok := tc.(*azopenai.ChatCompletionsFunctionToolCall); ok && funcTC.Function != nil {

				var params struct {
					FilePath string `json:"filePath"`
					DirPath  string `json:"dirPath"`
				}

				if err := json.Unmarshal([]byte(*funcTC.Function.Arguments), &params); err != nil {
					out := fmt.Sprintf("ERROR: Failed to parse tool arguments: %v", err)

					//If error occurs, create a tool message with the error so that the LLM can see it
					content := azopenai.NewChatRequestToolMessageContent(out)
					toolMsg := &azopenai.ChatRequestToolMessage{
						Content:    content,
						ToolCallID: funcTC.ID,
					}
					messages = append(messages, toolMsg)
					continue
				}

				var out string
				switch *funcTC.Function.Name {
				case "read_file":
					data, err := ReadFile(baseDir, params.FilePath)
					if err != nil {
						out = fmt.Sprintf("ERROR: %v", err)
					} else {
						out = data
					}
				case "list_directory":
					list, err := ListDirectory(baseDir, params.DirPath)
					if err != nil {
						out = fmt.Sprintf("ERROR: %v", err)
					} else {
						out = strings.Join(list, "\n")
					}
				case "file_exists":
					exists := FileExists(baseDir, params.FilePath)
					out = fmt.Sprintf("%v", exists)
				}

				// Create a tool message with the response
				content := azopenai.NewChatRequestToolMessageContent(out)
				toolMsg := &azopenai.ChatRequestToolMessage{
					Content:    content,
					ToolCallID: funcTC.ID,
				}
				messages = append(messages, toolMsg)
			}
		}

		// c) update opts and let the model produce its final reply
		opts.Messages = messages
	}

	return "", thisCallUsage, fmt.Errorf("maximum turns reached without final response")
}

// Does a GetChatCompletion but fills the promptText in %s and returns token usage
func (c *AzOpenAIClient) GetChatCompletionWithFormat(ctx context.Context, promptText string, args ...interface{}) (string, TokenUsage, error) {
	promptText = fmt.Sprintf(promptText, args...)
	return c.GetChatCompletion(ctx, promptText)
}

// Methods to implement mcptypes.AIAnalyzer interface directly

// Analyze implements mcptypes.AIAnalyzer
func (c *AzOpenAIClient) Analyze(ctx context.Context, prompt string) (string, error) {
	response, _, err := c.GetChatCompletion(ctx, prompt)
	return response, err
}

// AnalyzeWithFileTools implements mcptypes.AIAnalyzer
func (c *AzOpenAIClient) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	response, _, err := c.GetChatCompletionWithFileTools(ctx, prompt, baseDir)
	return response, err
}

// AnalyzeWithFormat implements mcptypes.AIAnalyzer
func (c *AzOpenAIClient) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	response, _, err := c.GetChatCompletionWithFormat(ctx, promptTemplate, args...)
	return response, err
}
