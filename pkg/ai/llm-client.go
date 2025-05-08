package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/container-copilot/pkg/logger"
)

type AzOpenAIClient struct {
	client       *azopenai.Client
	deploymentID string
	FileReader   FileReader // File reader for accessing repository files
}

func NewAzOpenAIClient(endpoint, apiKey, deploymentID string) (*AzOpenAIClient, error) {
	keyCredential := azcore.NewKeyCredential(apiKey)
	client, err := azopenai.NewClientWithKeyCredential(endpoint, keyCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating Azure OpenAI client: %v", err)
	}
	return &AzOpenAIClient{
		client:       client,
		deploymentID: deploymentID,
		FileReader:   nil,
	}, nil
}

func (c *AzOpenAIClient) SetFileReader(baseDir string) {
	c.FileReader = &DefaultFileReader{BaseDir: baseDir}
}

func (c *AzOpenAIClient) GetChatCompletion(ctx context.Context, promptText string) (string, error) {
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

func (c *AzOpenAIClient) GetChatCompletionWithFormat(ctx context.Context, promptText string, args ...interface{}) (string, error) {
	return c.GetChatCompletion(ctx, fmt.Sprintf(promptText, args...))
}

// GetChatCompletionWithFileTools sends a prompt with file‐system tools (read_file, list_directory, file_exists)
// under the 2023‑12‑01‑preview API (tools + tool_choice), and returns the final assistant reply.
func (c *AzOpenAIClient) GetChatCompletionWithFileTools(
	ctx context.Context,
	prompt, baseDir string,
) (string, error) {
	// 1) ensure we can read files
	if c.FileReader == nil {
		c.SetFileReader(baseDir)
	}

	// 2) get our file system tools
	tools := GetFileSystemTools()

	// 3) prime the messages with the user's prompt
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

	// 4) loop, handling any tool calls, up to N turns
	var final string
	for turn := range 15 {
		logger.Debugf("    tool calls turn %d", turn)
		resp, err := c.client.GetChatCompletions(ctx, opts, nil)
		if err != nil {
			return "", err
		}
		msg := resp.Choices[0].Message

		// Did the model invoke any tools?
		if tcalls := msg.ToolCalls; tcalls != nil && len(tcalls) > 0 {
			logger.Debugf("    invoked %d tools", len(tcalls))
			// a) echo the assistant's message with tool calls into our history
			var content *azopenai.ChatRequestAssistantMessageContent
			if msg.Content != nil {
				content = azopenai.NewChatRequestAssistantMessageContent(*msg.Content)
			}

			assistantMsg := &azopenai.ChatRequestAssistantMessage{
				Content: content,
			}

			// Build tool calls for the assistant message
			var toolCalls []azopenai.ChatCompletionsFunctionToolCall
			for _, tc := range tcalls {
				// Handle function specific tool calls
				if funcTC, ok := tc.(*azopenai.ChatCompletionsFunctionToolCall); ok && funcTC.Function != nil {
					toolCall := azopenai.ChatCompletionsFunctionToolCall{
						ID:   funcTC.ID,
						Type: funcTC.Type,
						Function: &azopenai.FunctionCall{
							Name:      funcTC.Function.Name,
							Arguments: funcTC.Function.Arguments,
						},
					}
					toolCalls = append(toolCalls, toolCall)
				}
			}

			// Add tool calls to assistant message
			var toolCallsClassification []azopenai.ChatCompletionsToolCallClassification
			for i := range toolCalls {
				toolCallsClassification = append(toolCallsClassification, &toolCalls[i])
			}
			assistantMsg.ToolCalls = toolCallsClassification
			messages = append(messages, assistantMsg)

			// b) execute each tool and append its response
			for _, tc := range tcalls {
				// Only process function tool calls
				if funcTC, ok := tc.(*azopenai.ChatCompletionsFunctionToolCall); ok && funcTC.Function != nil {
					var params struct {
						FilePath string `json:"filePath"`
						DirPath  string `json:"dirPath"`
					}
					_ = json.Unmarshal([]byte(*funcTC.Function.Arguments), &params)

					var out string
					switch *funcTC.Function.Name {
					case "read_file":
						data, err := c.FileReader.ReadFile(params.FilePath)
						if err != nil {
							out = fmt.Sprintf("ERROR: %v", err)
						} else {
							out = data
						}
					case "list_directory":
						list, err := c.FileReader.ListDirectory(params.DirPath)
						if err != nil {
							out = fmt.Sprintf("ERROR: %v", err)
						} else {
							out = strings.Join(list, "\n")
						}
					case "file_exists":
						exists := c.FileReader.FileExists(params.FilePath)
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
			continue
		}

		// no tool calls ⇒ final assistant content
		if msg.Content != nil {
			final = *msg.Content
			break
		}
	}

	if final == "" {
		return "", fmt.Errorf("no final response after handling tool calls")
	}
	return final, nil
}
