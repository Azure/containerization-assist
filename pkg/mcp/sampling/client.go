// Package sampling provides MCP sampling integration for LLM-powered features
package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/localrivet/gomcp/server"
)

// Client provides MCP sampling capabilities with Azure OpenAI fallback
type Client struct {
	mcpContext    *server.Context
	azureClient   *azopenai.Client
	logger        *slog.Logger
	maxTokens     int32
	temperature   float32
	topP          float32
	retryAttempts int
	tokenBudget   int // Token budget per retry to prevent runaway costs
}

// NewClient creates a new sampling client with optional Azure fallback
func NewClient(ctx *server.Context, logger *slog.Logger) *Client {
	client := &Client{
		mcpContext:    ctx,
		logger:        logger.With("component", "sampling-client"),
		maxTokens:     2048,
		temperature:   0.3, // Lower for deterministic responses
		topP:          0.9,
		retryAttempts: 3,
		tokenBudget:   5000, // Max tokens per retry session
	}

	// Initialize Azure OpenAI fallback if environment variables are set
	if key := os.Getenv("AZURE_OPENAI_KEY"); key != "" {
		endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
		if endpoint != "" {
			keyCredential := azcore.NewKeyCredential(key)
			azClient, err := azopenai.NewClientWithKeyCredential(endpoint, keyCredential, nil)
			if err != nil {
				logger.Warn("Failed to initialize Azure OpenAI client", "error", err)
			} else {
				client.azureClient = azClient
				logger.Info("Azure OpenAI fallback initialized")
			}
		}
	}

	return client
}

// SamplingRequest represents a request for LLM sampling
type SamplingRequest struct {
	Prompt       string
	MaxTokens    int32
	Temperature  float32
	SystemPrompt string
	Stream       bool
}

// SamplingResponse represents the LLM response
type SamplingResponse struct {
	Content    string
	TokensUsed int
	Model      string
	StopReason string
	Error      error
}

// Sample performs a synchronous LLM sampling request
func (c *Client) Sample(ctx context.Context, request SamplingRequest) (*SamplingResponse, error) {
	start := time.Now()
	defer func() {
		c.logger.Debug("Sampling completed",
			"duration", time.Since(start),
			"prompt_length", len(request.Prompt))
	}()

	// Adjust parameters if provided
	maxTokens := c.maxTokens
	if request.MaxTokens > 0 {
		maxTokens = request.MaxTokens
	}

	temperature := c.temperature
	if request.Temperature > 0 {
		temperature = request.Temperature
	}

	// MCP sampling not yet implemented - fallback to Azure OpenAI
	c.logger.Debug("MCP sampling not implemented, using Azure OpenAI fallback")

	// Fallback to Azure OpenAI if available
	if c.azureClient != nil {
		return c.sampleWithAzure(ctx, request, maxTokens, temperature)
	}

	// No sampling available
	return nil, fmt.Errorf("no sampling capability available (MCP not supported and Azure OpenAI not configured)")
}

// sampleWithMCP uses the MCP protocol for sampling (not yet implemented)
func (c *Client) sampleWithMCP(ctx context.Context, request SamplingRequest, maxTokens int32, temperature float32) (*SamplingResponse, error) {
	// TODO: Implement MCP sampling when the gomcp library supports it
	return nil, fmt.Errorf("MCP sampling not yet implemented")
}

// sampleWithAzure uses Azure OpenAI as fallback
func (c *Client) sampleWithAzure(ctx context.Context, request SamplingRequest, maxTokens int32, temperature float32) (*SamplingResponse, error) {
	messages := []azopenai.ChatRequestMessageClassification{
		&azopenai.ChatRequestUserMessage{
			Content: azopenai.NewChatRequestUserMessageContent(request.Prompt),
		},
	}

	// Add system prompt if provided
	if request.SystemPrompt != "" {
		systemMsg := &azopenai.ChatRequestSystemMessage{
			Content: azopenai.NewChatRequestSystemMessageContent(request.SystemPrompt),
		}
		messages = append([]azopenai.ChatRequestMessageClassification{systemMsg}, messages...)
	}

	deploymentName := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	if deploymentName == "" {
		deploymentName = "gpt-4" // Default deployment
	}

	options := azopenai.ChatCompletionsOptions{
		Messages:    messages,
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
		TopP:        &c.topP,
	}

	resp, err := c.azureClient.GetChatCompletions(ctx, options, nil)
	if err != nil {
		return nil, fmt.Errorf("Azure OpenAI request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices from Azure OpenAI")
	}

	choice := resp.Choices[0]
	content := ""
	if choice.Message != nil && choice.Message.Content != nil {
		content = *choice.Message.Content
	}

	tokenUsage := 0
	if resp.Usage != nil {
		tokenUsage = int(*resp.Usage.TotalTokens)
	}

	return &SamplingResponse{
		Content:    content,
		TokensUsed: tokenUsage,
		Model:      deploymentName,
		StopReason: string(*choice.FinishReason),
	}, nil
}

// AnalyzeError uses LLM to analyze an error and suggest fixes
func (c *Client) AnalyzeError(ctx context.Context, operation string, err error, context string) (*ErrorAnalysis, error) {
	prompt := fmt.Sprintf(`Analyze this containerization error and suggest fixes:

Operation: %s
Error: %v
Context: %s

Provide a structured analysis with:
1. Root cause analysis (be specific about what went wrong)
2. Step-by-step fix instructions (actionable commands/code)
3. Alternative approaches if the direct fix doesn't work
4. Prevention strategies for the future

Format your response as:
ROOT CAUSE:
<explanation>

FIX STEPS:
1. <step>
2. <step>
...

ALTERNATIVES:
- <alternative approach>
...

PREVENTION:
- <strategy>
...`, operation, err, context)

	request := SamplingRequest{
		Prompt:       prompt,
		MaxTokens:    1500,
		Temperature:  0.3,
		SystemPrompt: "You are a containerization expert helping to diagnose and fix Docker and Kubernetes deployment issues.",
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze error: %w", err)
	}

	// Check token budget
	if response.TokensUsed > c.tokenBudget {
		c.logger.Warn("Token budget exceeded",
			"used", response.TokensUsed,
			"budget", c.tokenBudget)
	}

	return parseErrorAnalysis(response.Content), nil
}

// ErrorAnalysis represents structured error analysis from LLM
type ErrorAnalysis struct {
	RootCause    string
	FixSteps     []string
	Alternatives []string
	Prevention   []string
	CanAutoFix   bool
}

// parseErrorAnalysis parses the LLM response into structured format
func parseErrorAnalysis(content string) *ErrorAnalysis {
	analysis := &ErrorAnalysis{
		FixSteps:     []string{},
		Alternatives: []string{},
		Prevention:   []string{},
	}

	// Simple parsing - in production, use a more robust parser
	currentSection := ""

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "ROOT CAUSE:") {
			currentSection = "root"
			continue
		} else if strings.HasPrefix(line, "FIX STEPS:") {
			currentSection = "fix"
			continue
		} else if strings.HasPrefix(line, "ALTERNATIVES:") {
			currentSection = "alt"
			continue
		} else if strings.HasPrefix(line, "PREVENTION:") {
			currentSection = "prev"
			continue
		}

		if line == "" {
			continue
		}

		switch currentSection {
		case "root":
			if analysis.RootCause == "" {
				analysis.RootCause = line
			} else {
				analysis.RootCause += " " + line
			}
		case "fix":
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				analysis.FixSteps = append(analysis.FixSteps, strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
			} else if matched := strings.TrimPrefix(line, "1. "); matched != line {
				analysis.FixSteps = append(analysis.FixSteps, matched)
			} else if matched := strings.TrimPrefix(line, "2. "); matched != line {
				analysis.FixSteps = append(analysis.FixSteps, matched)
			}
		case "alt":
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				analysis.Alternatives = append(analysis.Alternatives, strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
			}
		case "prev":
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				analysis.Prevention = append(analysis.Prevention, strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
			}
		}
	}

	// Determine if we can auto-fix (simple heuristic)
	analysis.CanAutoFix = len(analysis.FixSteps) > 0 &&
		strings.Contains(strings.ToLower(analysis.RootCause), "missing") ||
		strings.Contains(strings.ToLower(analysis.RootCause), "incorrect")

	return analysis
}

// GenerateDockerfile uses LLM to generate an optimized Dockerfile
func (c *Client) GenerateDockerfile(ctx context.Context, language, framework string, port int) (string, error) {
	prompt := fmt.Sprintf(`Generate a production-ready, multi-stage Dockerfile for:
- Language: %s
- Framework: %s  
- Port: %d

Requirements:
1. Use multi-stage build for minimal final image size
2. Implement proper layer caching
3. Use non-root user for security
4. Include health checks
5. Handle signals properly (SIGTERM)
6. Separate build-time ARGs from runtime ENVs
7. Include security scanning step

Provide only the Dockerfile content without explanation.`, language, framework, port)

	request := SamplingRequest{
		Prompt:      prompt,
		MaxTokens:   1000,
		Temperature: 0.2, // Lower for more consistent output
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	return response.Content, nil
}

// SetTokenBudget sets the maximum tokens allowed per retry session
func (c *Client) SetTokenBudget(budget int) {
	c.tokenBudget = budget
}

// GetTokenBudget returns the current token budget
func (c *Client) GetTokenBudget() int {
	return c.tokenBudget
}
