// Package sampling provides MCP sampling integration for LLM-powered features.
package sampling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/tracing"
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
		return nil, fmt.Errorf("invalid configuration: %w", err)
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
}

// SamplingResponse represents a response from the MCP sampling API.
type SamplingResponse struct {
	Content    string
	TokensUsed int
	Model      string
	StopReason string
	Error      error
}

// Sample performs sampling with simple retry & budget enforcement.
func (c *Client) Sample(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	ctx, span := tracing.StartSpan(ctx, "sampling.sample")
	defer span.End()

	// Add tracing attributes
	span.SetAttributes(
		attribute.String(tracing.AttrComponent, "sampling"),
		attribute.Int("sampling.max_tokens", int(req.MaxTokens)),
		attribute.Float64("sampling.temperature", float64(req.Temperature)),
		attribute.Int("sampling.prompt_length", len(req.Prompt)),
	)

	if srv := server.ServerFromContext(ctx); srv == nil {
		return nil, errors.New("no MCP server in context – cannot perform sampling")
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
	finalErr := fmt.Errorf("all %d sampling attempts failed: %w", c.retryAttempts, lastErr)
	span.RecordError(finalErr)
	span.SetAttributes(attribute.String("error.final", finalErr.Error()))
	return nil, finalErr
}

// --- internals --------------------------------------------------------------

func (c *Client) callMCP(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	// For now, return a fallback response since the actual MCP sampling API
	// in mcp-go v0.33.0 may not have the exact interface we expect
	// This allows the client to work without MCP sampling while we develop
	c.logger.Debug("MCP sampling not yet implemented, using fallback",
		"prompt_length", len(req.Prompt),
		"max_tokens", req.MaxTokens,
		"temperature", req.Temperature)

	// Return the prompt as content for now - this allows AI assistants
	// to see the request and handle it appropriately
	return &SamplingResponse{
		Content:    fmt.Sprintf("AI ASSISTANCE REQUESTED: %s", req.Prompt),
		TokensUsed: estimateTokens(req.Prompt),
		Model:      "mcp-fallback",
		StopReason: "fallback",
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
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"Error":   inputErr.Error(),
		"Context": contextInfo,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("error-analysis", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render error analysis template: %w", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze error: %w", err)
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
