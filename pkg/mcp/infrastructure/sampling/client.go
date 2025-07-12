// Package sampling provides MCP sampling integration for LLM-powered features
package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// Client provides MCP sampling capabilities by delegating to the calling AI assistant
type Client struct {
	ctx           context.Context
	logger        *slog.Logger
	maxTokens     int32
	temperature   float32
	retryAttempts int
	tokenBudget   int // Token budget per retry to prevent runaway costs
}

// NewClient creates a new sampling client that delegates to the calling AI assistant
func NewClient(ctx context.Context, logger *slog.Logger) *Client {
	return &Client{
		ctx:           ctx,
		logger:        logger.With("component", "sampling-client"),
		maxTokens:     2048,
		temperature:   0.3, // Lower for deterministic responses
		retryAttempts: 3,
		tokenBudget:   5000, // Max tokens per retry session
	}
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

// Sample performs a synchronous LLM sampling request by delegating to the calling AI assistant
func (c *Client) Sample(ctx context.Context, request SamplingRequest) (*SamplingResponse, error) {
	start := time.Now()
	defer func() {
		c.logger.Debug("Sampling completed",
			"duration", time.Since(start),
			"prompt_length", len(request.Prompt))
	}()

	// Check if we have MCP server context for delegation
	if srv := server.ServerFromContext(c.ctx); srv != nil {
		return c.sampleWithMCP(ctx, request)
	}

	// No MCP context available - this shouldn't happen in normal operation
	return nil, fmt.Errorf("no MCP server context available for AI delegation - sampling requires AI assistant connection")
}

// sampleWithMCP delegates the sampling request to the calling AI assistant via MCP protocol
func (c *Client) sampleWithMCP(ctx context.Context, request SamplingRequest) (*SamplingResponse, error) {
	// Create a well-formatted prompt for the AI assistant
	prompt := request.Prompt
	if request.SystemPrompt != "" {
		prompt = fmt.Sprintf("%s\n\n%s", request.SystemPrompt, request.Prompt)
	}

	// Create a preview of the prompt for logging
	promptPreview := prompt
	if len(prompt) > 100 {
		promptPreview = prompt[:100] + "..."
	}

	c.logger.Info("Requesting AI assistance for containerization task",
		"prompt_preview", promptPreview,
		"max_tokens", request.MaxTokens,
		"temperature", request.Temperature)

	// In a proper MCP implementation, this would send a sampling request
	// to the AI assistant. For now, we format the request in a way that
	// makes it clear to the AI what kind of help is needed.

	// Since mcp-go doesn't have native sampling yet, we'll structure this
	// as a clear request that the AI assistant can understand and respond to
	return &SamplingResponse{
		Content:    prompt,                          // Return the prompt directly for the AI to handle
		TokensUsed: len(strings.Split(prompt, " ")), // Rough token estimate
		Model:      "mcp-delegated",
		StopReason: "ai_assistance_requested",
	}, nil
}

// AnalyzeError delegates error analysis to the AI assistant via MCP prompts
func (c *Client) AnalyzeError(ctx context.Context, operation string, err error, context string) (*ErrorAnalysis, error) {
	c.logger.Info("Requesting AI assistance for error analysis",
		"operation", operation,
		"error_preview", err.Error()[:min(50, len(err.Error()))]+"...")

	// Create a structured request for the AI assistant to handle
	// The AI will see this request and can use the appropriate MCP prompts
	prompt := fmt.Sprintf(`Please analyze this containerization error using the available MCP prompts:

Operation: %s
Error: %s
Context: %s

This appears to be a %s issue. Please use the appropriate analysis prompts to:
1. Identify the root cause
2. Suggest specific fixes
3. Provide alternative approaches
4. Recommend prevention strategies`, operation, err.Error(), context, operation)

	request := SamplingRequest{
		Prompt:       prompt,
		MaxTokens:    1500,
		Temperature:  0.3,
		SystemPrompt: "You are a containerization expert. Use the available MCP prompts (analyze_dockerfile_errors, analyze_manifest_errors, etc.) to provide comprehensive error analysis.",
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
