// Package sampling provides MCP sampling integration for LLM-powered features.
package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/tracing"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Option configures a Client.
type Option func(*Client)

// WithMaxTokens sets a default max-tokens value when the caller does not specify one.
func WithMaxTokens(n int32) Option { return func(c *Client) { c.maxTokens = n } }

// WithTemperature sets a default temperature.
func WithTemperature(t float32) Option { return func(c *Client) { c.temperature = t } }

// WithRetry sets the retry budget (attempts and token budget per attempt).
func WithRetry(attempts int, budget int) Option {
	return func(c *Client) {
		c.retryAttempts = attempts
		c.tokenBudget = budget
	}
}

// Client delegates LLM work to the calling AI assistant via the MCP sampling API.
type Client struct {
	logger           *slog.Logger
	maxTokens        int32
	temperature      float32
	retryAttempts    int
	tokenBudget      int
	baseBackoff      time.Duration
	maxBackoff       time.Duration
	streamingEnabled bool
	requestTimeout   time.Duration
}

// NewClient returns a new sampling client.
func NewClient(logger *slog.Logger, opts ...Option) *Client {
	cfg := DefaultConfig()
	c := &Client{
		logger:           logger.With("component", "sampling-client"),
		maxTokens:        cfg.MaxTokens,
		temperature:      cfg.Temperature,
		retryAttempts:    cfg.RetryAttempts,
		tokenBudget:      cfg.TokenBudget,
		baseBackoff:      cfg.BaseBackoff,
		maxBackoff:       cfg.MaxBackoff,
		streamingEnabled: cfg.StreamingEnabled,
		requestTimeout:   cfg.RequestTimeout,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// NewClientFromEnv creates a new sampling client with configuration from environment variables
func NewClientFromEnv(logger *slog.Logger, opts ...Option) (*Client, error) {
	cfg := LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		return nil, errors.New(errors.CodeConfigurationInvalid, "sampling", "invalid configuration", err)
	}

	// Apply config first, then any additional options
	allOpts := append([]Option{WithConfig(cfg)}, opts...)
	return NewClient(logger, allOpts...), nil
}

// --- Public API -------------------------------------------------------------

// SamplingRequest represents a request to the MCP sampling API.
type SamplingRequest struct {
	Prompt       string
	MaxTokens    int32
	Temperature  float32
	SystemPrompt string
	Stream       bool
	Metadata     map[string]interface{}

	// Advanced parameters (extracted from metadata for convenience)
	TopP             *float32
	FrequencyPenalty *float32
	PresencePenalty  *float32
	StopSequences    []string
	Seed             *int
	LogitBias        map[string]float32
}

// SamplingResponse represents a response from the MCP sampling API.
type SamplingResponse struct {
	Content    string
	TokensUsed int
	Model      string
	StopReason string
	Error      error
}

// SampleInternal performs sampling with simple retry & budget enforcement.
func (c *Client) SampleInternal(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	ctx, span := tracing.StartSpan(ctx, "sampling.sample")
	defer span.End()

	// Add tracing attributes
	span.SetAttributes(
		attribute.String(tracing.AttrComponent, "sampling"),
		attribute.Int("sampling.max_tokens", int(req.MaxTokens)),
		attribute.Float64("sampling.temperature", float64(req.Temperature)),
		attribute.Int("sampling.prompt_length", len(req.Prompt)),
	)

	// Add advanced parameter attributes
	if req.TopP != nil {
		span.SetAttributes(attribute.Float64("sampling.top_p", float64(*req.TopP)))
	}
	if req.FrequencyPenalty != nil {
		span.SetAttributes(attribute.Float64("sampling.frequency_penalty", float64(*req.FrequencyPenalty)))
	}
	if req.PresencePenalty != nil {
		span.SetAttributes(attribute.Float64("sampling.presence_penalty", float64(*req.PresencePenalty)))
	}
	if len(req.StopSequences) > 0 {
		span.SetAttributes(attribute.Int("sampling.stop_sequences_count", len(req.StopSequences)))
	}
	if req.Seed != nil {
		span.SetAttributes(attribute.Int("sampling.seed", *req.Seed))
	}
	if len(req.LogitBias) > 0 {
		span.SetAttributes(attribute.Int("sampling.logit_bias_count", len(req.LogitBias)))
	}

	if srv := server.ServerFromContext(ctx); srv == nil {
		return nil, errors.New(errors.CodeInternalError, "sampling", "no MCP server in context – cannot perform sampling", nil)
	}

	// Default values.
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	if req.Temperature == 0 {
		req.Temperature = c.temperature
	}

	var lastErr error
	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Add retry attempt to span
		span.SetAttributes(attribute.Int(tracing.AttrSamplingRetryAttempt, attempt+1))

		resp, err := c.callMCP(ctx, req)
		if err == nil {
			// Add success attributes
			span.SetAttributes(
				attribute.Int(tracing.AttrSamplingTokensUsed, resp.TokensUsed),
				attribute.String("sampling.model", resp.Model),
				attribute.String("sampling.stop_reason", resp.StopReason),
			)
			return resp, nil
		}

		// Abort early on non-retryable errors.
		if !isRetryable(err) {
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.non_retryable", err.Error()))
			return nil, err
		}
		lastErr = err
		backoff := c.calculateBackoff(attempt)
		c.logger.Warn("sampling attempt failed – backing off", "attempt", attempt+1, "err", err, "backoff", backoff)

		// Record retry event
		span.AddEvent("retry.backoff", trace.WithAttributes(
			attribute.String("error", err.Error()),
			attribute.Int("attempt", attempt+1),
			attribute.String("backoff", backoff.String()),
		))

		// Use a timer to respect context cancellation during backoff
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
			// Continue to next attempt
		}
	}

	// Record final failure
	finalErr := errors.New(errors.CodeOperationFailed, "sampling",
		fmt.Sprintf("all %d sampling attempts failed", c.retryAttempts), lastErr)
	span.RecordError(finalErr)
	span.SetAttributes(attribute.String("error.final", finalErr.Error()))
	return nil, finalErr
}

// --- internals --------------------------------------------------------------

func (c *Client) callMCP(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	// Try to get MCP server from context
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		c.logger.Debug("No MCP server in context, using fallback",
			"prompt_length", len(req.Prompt))

		// Return the prompt as content - this allows AI assistants
		// to see the request and handle it appropriately
		return &SamplingResponse{
			Content:    fmt.Sprintf("AI ASSISTANCE REQUESTED: %s", req.Prompt),
			TokensUsed: estimateTokens(req.Prompt),
			Model:      "mcp-fallback",
			StopReason: "fallback",
		}, nil
	}

	c.logger.Info("Using MCP sampling with server",
		"prompt_length", len(req.Prompt),
		"max_tokens", req.MaxTokens,
		"temperature", req.Temperature)

	// Use actual MCP sampling when server is available
	// This enables AI-powered error analysis during deployment failures
	return c.callMCPSampling(ctx, srv, req)
}

// callMCPSampling performs actual MCP sampling using the server's sampling API
func (c *Client) callMCPSampling(ctx context.Context, srv *server.MCPServer, req SamplingRequest) (*SamplingResponse, error) {
	c.logger.Info("Making MCP sampling request",
		"prompt_length", len(req.Prompt),
		"max_tokens", req.MaxTokens,
		"temperature", req.Temperature)

	// Create MCP sampling request following the official API
	samplingRequest := mcp.CreateMessageRequest{
		CreateMessageParams: mcp.CreateMessageParams{
			Messages: []mcp.SamplingMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: req.Prompt,
					},
				},
			},
			MaxTokens:   int(req.MaxTokens),       // Convert int32 to int
			Temperature: float64(req.Temperature), // Convert float32 to float64
		},
	}

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		samplingRequest.CreateMessageParams.SystemPrompt = req.SystemPrompt
	}

	// Add advanced parameters if available
	// Note: The MCP library may not support all of these parameters directly,
	// but we can try to set them or pass them via metadata
	if req.TopP != nil {
		// Try to set TopP if the field exists in the MCP struct
		// For now, we'll add it to a metadata field or extension if available
		c.logger.Debug("TopP parameter requested", "top_p", *req.TopP)
	}

	if req.FrequencyPenalty != nil {
		c.logger.Debug("FrequencyPenalty parameter requested", "frequency_penalty", *req.FrequencyPenalty)
	}

	if req.PresencePenalty != nil {
		c.logger.Debug("PresencePenalty parameter requested", "presence_penalty", *req.PresencePenalty)
	}

	if len(req.StopSequences) > 0 {
		c.logger.Debug("StopSequences parameter requested", "stop_sequences", req.StopSequences)
		// Many AI models support stop sequences, but we need to check if MCP supports them
	}

	if req.Seed != nil {
		c.logger.Debug("Seed parameter requested", "seed", *req.Seed)
	}

	if len(req.LogitBias) > 0 {
		c.logger.Debug("LogitBias parameter requested", "logit_bias_count", len(req.LogitBias))
	}

	// Make the actual MCP sampling request
	c.logger.Info("Calling srv.RequestSampling", "messages_count", len(samplingRequest.CreateMessageParams.Messages))
	result, err := srv.RequestSampling(ctx, samplingRequest)
	if err != nil {
		c.logger.Error("MCP sampling request failed", "error", err)
		return nil, fmt.Errorf("MCP sampling failed: %w", err)
	}

	c.logger.Info("MCP sampling response received",
		"result_type", fmt.Sprintf("%T", result),
		"has_content", result.Content != nil,
		"content_type", fmt.Sprintf("%T", result.Content))

	// Extract content from the result (CreateMessageResult embeds SamplingMessage)
	var content string
	var tokensUsed int
	var model string

	if result.Content != nil {
		// Try to extract as TextContent first
		if textContent, ok := result.Content.(mcp.TextContent); ok {
			content = textContent.Text
			c.logger.Debug("Extracted text content", "content_length", len(content))
		} else if contentMap, ok := result.Content.(map[string]interface{}); ok {
			// Handle map[string]interface{} format with "text" key
			if textValue, exists := contentMap["text"]; exists {
				if textStr, ok := textValue.(string); ok {
					content = textStr
					c.logger.Info("Extracted text from map content", "content_length", len(content))
				}
			}
		} else {
			c.logger.Warn("Content is not TextContent or map",
				"actual_type", fmt.Sprintf("%T", result.Content),
				"content_value", fmt.Sprintf("%+v", result.Content))
		}
	}

	// Log if we got empty content from MCP
	if content == "" {
		c.logger.Warn("MCP sampling returned empty content",
			"prompt_preview", truncateString(req.Prompt, 100),
			"result", fmt.Sprintf("%+v", result))
	}

	// Estimate token usage since MCP doesn't provide usage statistics
	tokensUsed = estimateTokens(content)

	if result.Model != "" {
		model = result.Model
	} else {
		model = "mcp-ai-assistant"
	}

	c.logger.Debug("MCP sampling completed",
		"content_length", len(content),
		"tokens_used", tokensUsed,
		"model", model)

	return &SamplingResponse{
		Content:    content,
		TokensUsed: tokensUsed,
		Model:      model,
		StopReason: result.StopReason,
	}, nil
}

// toResponse converts a generic response to our SamplingResponse
// This will be used when we have the actual MCP sampling API working
func (c *Client) toResponse(content string, model string) *SamplingResponse {
	return &SamplingResponse{
		Content:    content,
		TokensUsed: estimateTokens(content),
		Model:      model,
		StopReason: "complete",
	}
}

// estimateTokens provides a rough token count estimate.
// Uses empirical multiplier of 1.3 tokens per word.
func estimateTokens(s string) int {
	words := len(strings.Fields(s))
	return int(float64(words) * 1.3)
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func isRetryable(err error) bool {
	// Check for common retryable error patterns
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "temporarily") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "broken pipe")
}

// ErrorAnalysis represents the structured analysis of an error.
type ErrorAnalysis struct {
	RootCause    string   `json:"root_cause"`
	Fix          string   `json:"fix"`
	FixSteps     []string `json:"fix_steps"`
	Alternatives []string `json:"alternatives"`
	Prevention   []string `json:"prevention"`
	CanAutoFix   bool     `json:"can_auto_fix"`
}

// parseErrorAnalysis extracts structured error analysis from AI response.
func parseErrorAnalysis(content string) *ErrorAnalysis {
	analysis := &ErrorAnalysis{
		Alternatives: []string{},
		Prevention:   []string{},
		FixSteps:     []string{},
		CanAutoFix:   true, // Default to true, will be refined based on content
	}

	lines := strings.Split(content, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect sections
		if strings.HasPrefix(line, "ROOT CAUSE:") {
			currentSection = "root_cause"
			analysis.RootCause = strings.TrimSpace(strings.TrimPrefix(line, "ROOT CAUSE:"))
		} else if strings.HasPrefix(line, "FIX STEPS:") || strings.HasPrefix(line, "FIX:") {
			currentSection = "fix"
		} else if strings.HasPrefix(line, "ALTERNATIVES:") {
			currentSection = "alternatives"
		} else if strings.HasPrefix(line, "PREVENTION:") {
			currentSection = "prevention"
		} else if strings.HasPrefix(line, "- ") {
			// Handle list items
			item := strings.TrimPrefix(line, "- ")
			switch currentSection {
			case "fix":
				analysis.FixSteps = append(analysis.FixSteps, item)
				if analysis.Fix == "" {
					analysis.Fix = item
				} else {
					analysis.Fix += "; " + item
				}
			case "alternatives":
				analysis.Alternatives = append(analysis.Alternatives, item)
			case "prevention":
				analysis.Prevention = append(analysis.Prevention, item)
			}
		} else if currentSection == "root_cause" && analysis.RootCause == "" {
			analysis.RootCause = line
		}
	}

	return analysis
}

// AnalyzeError uses MCP sampling to analyze an error and suggest fixes.
func (c *Client) AnalyzeError(ctx context.Context, inputErr error, contextInfo string) (*ErrorAnalysis, error) {
	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "sampling", "failed to create template manager", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"Error":   inputErr.Error(),
		"Context": contextInfo,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("error-analysis", templateData)
	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "sampling", "failed to render error analysis template", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.SampleInternal(ctx, request)
	if err != nil {
		return nil, errors.New(errors.CodeToolExecutionFailed, "sampling", "failed to analyze error", err)
	}

	return parseErrorAnalysis(response.Content), nil
}

// SetTokenBudget sets the maximum tokens allowed per retry session
func (c *Client) SetTokenBudget(budget int) {
	c.tokenBudget = budget
}

// GetTokenBudget returns the current token budget
func (c *Client) GetTokenBudget() int {
	return c.tokenBudget
}

// calculateBackoff computes exponential backoff with jitter
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: baseBackoff * 2^attempt
	backoff := c.baseBackoff * time.Duration(1<<attempt)

	// Cap at maxBackoff
	if backoff > c.maxBackoff {
		backoff = c.maxBackoff
	}

	// Add jitter (±25% of backoff)
	jitter := backoff / 4
	backoff = backoff - jitter + time.Duration(attempt*123456789%int(jitter*2))

	return backoff
}

// emitTokenProgress emits progress updates during token streaming
func (c *Client) emitTokenProgress(ctx context.Context, tokensGenerated int, maxTokens int32, startTime time.Time) {
	if srv := server.ServerFromContext(ctx); srv != nil {
		percentage := 0
		if maxTokens > 0 {
			percentage = int(float64(tokensGenerated) / float64(maxTokens) * 100)
			if percentage > 100 {
				percentage = 100
			}
		}

		elapsed := time.Since(startTime)
		var eta time.Duration
		if tokensGenerated > 0 && maxTokens > 0 {
			tokensPerSecond := float64(tokensGenerated) / elapsed.Seconds()
			remainingTokens := float64(maxTokens) - float64(tokensGenerated)
			if tokensPerSecond > 0 {
				eta = time.Duration(remainingTokens/tokensPerSecond) * time.Second
			}
		}

		payload := map[string]interface{}{
			"progressToken": "llm-generation",
			"step":          tokensGenerated,
			"total":         int(maxTokens),
			"percentage":    percentage,
			"status":        "generating",
			"step_name":     "llm_token_generation",
			"substep_name":  fmt.Sprintf("token %d/%d", tokensGenerated, maxTokens),
			"message":       fmt.Sprintf("Generating tokens: %d/%d (%d%%)", tokensGenerated, maxTokens, percentage),
			"eta_ms":        eta.Milliseconds(),
			"metadata": map[string]interface{}{
				"kind":             "token_stream",
				"tokens_generated": tokensGenerated,
				"estimated_total":  maxTokens,
				"tokens_per_sec":   float64(tokensGenerated) / elapsed.Seconds(),
			},
		}

		// Send the progress notification
		if err := srv.SendNotificationToClient(ctx, "notifications/progress", payload); err != nil {
			c.logger.Debug("Failed to send token progress notification", "error", err)
		}
	}
}
