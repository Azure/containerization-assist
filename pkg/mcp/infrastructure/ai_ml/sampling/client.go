package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Option func(*Client)

// WithMaxTokens sets a default max-tokens value when the caller does not specify one.
func WithMaxTokens(n int32) Option { return func(c *Client) { c.maxTokens = n } }

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

type SamplingResponse struct {
	Content    string
	TokensUsed int
	Model      string
	StopReason string
	Error      error
}

// SampleInternal performs sampling with AI-assisted retry and error correction.
func (c *Client) SampleInternal(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	start := time.Now()

	// Create enhanced logger for structured LLM logging
	enhancedLogger := NewEnhancedLogger(c.logger)
	reqLogger := enhancedLogger.WithRequestContext(c.logger, req)

	// Log detailed request information
	enhancedLogger.LogLLMRequest(ctx, reqLogger, req)

	// Default values.
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	if req.Temperature == 0 {
		req.Temperature = c.temperature
	}

	// Use AI-assisted retry for better error recovery
	return c.sampleWithAIAssist(ctx, req, enhancedLogger, reqLogger, start)
}

// sampleWithAIAssist performs AI-assisted sampling with intelligent error correction
func (c *Client) sampleWithAIAssist(ctx context.Context, originalReq SamplingRequest, enhancedLogger *EnhancedLogger, reqLogger *slog.Logger, start time.Time) (*SamplingResponse, error) {
	var lastErr error
	var errorHistory []string
	currentReq := originalReq // Start with original request

	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check for MCP server context on each attempt
		var resp *SamplingResponse
		var err error
		if srv := server.ServerFromContext(ctx); srv == nil {
			err = errors.New(errors.CodeInternalError, "sampling", "no MCP server in context – cannot perform sampling", nil)
		} else {
			resp, err = c.callMCP(ctx, currentReq)
		}

		if err == nil {
			// Log successful response with enhanced details
			enhancedLogger.LogLLMResponse(ctx, reqLogger, currentReq, resp, time.Since(start))
			return resp, nil
		}

		// Log error with enhanced context
		enhancedLogger.LogLLMError(ctx, reqLogger, currentReq, err, time.Since(start), attempt+1)

		// Abort early on non-retryable errors.
		if !IsRetryable(err) {
			return nil, err
		}

		lastErr = err
		errorHistory = append(errorHistory, err.Error())

		// If this isn't the last attempt, try to use AI to improve the request
		if attempt < c.retryAttempts-1 {
			c.logger.Info("Attempting AI-assisted error correction", "attempt", attempt+1, "error", err.Error())

			improvedReq, correctionErr := c.applyAICorrection(ctx, currentReq, errorHistory, attempt+1)
			if correctionErr != nil {
				c.logger.Warn("AI correction failed, using original request", "error", correctionErr)
				// Continue with current request
			} else {
				c.logger.Info("Applied AI corrections to request", "attempt", attempt+1)
				currentReq = improvedReq
			}
		}

		backoff := c.calculateBackoff(attempt)
		c.logger.Warn("sampling attempt failed – backing off", "attempt", attempt+1, "err", err, "backoff", backoff)

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
	return nil, finalErr
}

func (c *Client) callMCP(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	// Try to get MCP server from context
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		c.logger.Debug("No MCP server in context, sampling unavailable",
			"prompt_length", len(req.Prompt))

		// Return proper structured error
		return nil, errors.New(errors.CodeDisabled, "sampling",
			"MCP server not available for AI sampling - ensure proper context initialization", nil)
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
		return nil, errors.New(errors.CodeOperationFailed, "sampling",
			"MCP sampling request failed", err)
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
	// Check if MCP server is available for AI analysis
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		c.logger.Warn("No MCP server available for AI error analysis, using pattern-based analysis")
		return c.createPatternBasedErrorAnalysis(inputErr.Error(), contextInfo), nil
	}

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		c.logger.Warn("Failed to create template manager, falling back to pattern-based analysis", "error", err)
		return c.createPatternBasedErrorAnalysis(inputErr.Error(), contextInfo), nil
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"Error":   inputErr.Error(),
		"Context": contextInfo,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("error-analysis", templateData)
	if err != nil {
		c.logger.Warn("Failed to render error analysis template, falling back to pattern-based analysis", "error", err)
		return c.createPatternBasedErrorAnalysis(inputErr.Error(), contextInfo), nil
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.callMCP(ctx, request)
	if err != nil {
		c.logger.Warn("MCP sampling failed for error analysis, falling back to pattern-based analysis", "error", err)
		return c.createPatternBasedErrorAnalysis(inputErr.Error(), contextInfo), nil
	}

	// Parse AI response, but fallback to pattern-based if parsing fails
	analysis := parseErrorAnalysis(response.Content)
	if analysis.RootCause == "" && len(analysis.FixSteps) == 0 {
		c.logger.Warn("AI analysis returned empty results, falling back to pattern-based analysis")
		return c.createPatternBasedErrorAnalysis(inputErr.Error(), contextInfo), nil
	}

	return analysis, nil
}

// createPatternBasedErrorAnalysis creates error analysis using pattern recognition
// This serves as a fallback when MCP sampling is unavailable
func (c *Client) createPatternBasedErrorAnalysis(errorMsg, contextInfo string) *ErrorAnalysis {
	analysis := &ErrorAnalysis{
		Alternatives: []string{},
		Prevention:   []string{},
		FixSteps:     []string{},
		CanAutoFix:   true,
	}

	errorLower := strings.ToLower(errorMsg)

	// Maven-related error patterns
	if strings.Contains(errorLower, "mvn") && strings.Contains(errorLower, "command not found") {
		analysis.RootCause = "Maven is not installed in the Docker container"
		analysis.FixSteps = []string{
			"Use maven:3.9-eclipse-temurin-17 as base image",
			"Or install Maven in Dockerfile: RUN apt-get update && apt-get install -y maven",
		}
		analysis.Fix = "Install Maven in Docker container"
		analysis.Alternatives = []string{"Use Gradle instead of Maven", "Use multi-stage build with Maven"}
		analysis.Prevention = []string{"Always use Maven-enabled base images for Java projects"}
		return analysis
	}

	// Gradle-related error patterns
	if strings.Contains(errorLower, "gradle") && strings.Contains(errorLower, "command not found") {
		analysis.RootCause = "Gradle is not installed in the Docker container"
		analysis.FixSteps = []string{
			"Use gradle:8-jdk17 as base image",
			"Or install Gradle in Dockerfile with proper installation commands",
		}
		analysis.Fix = "Install Gradle in Docker container"
		analysis.Alternatives = []string{"Use Maven instead of Gradle", "Use Gradle wrapper"}
		analysis.Prevention = []string{"Always use Gradle-enabled base images for Gradle projects"}
		return analysis
	}

	// Kubernetes deployment patterns
	if strings.Contains(errorLower, "pods ready") && strings.Contains(errorLower, "deployment validation") {
		analysis.RootCause = "Pod is not becoming ready, likely due to image pull or application startup issues"
		analysis.FixSteps = []string{
			"Check if image exists and is accessible",
			"Verify port configuration in application and Kubernetes manifests",
			"Add proper health checks and readiness probes",
			"Wait longer for image pull and application startup",
		}
		analysis.Fix = "Fix pod readiness and deployment configuration"
		analysis.Alternatives = []string{"Use different image tag", "Simplify deployment configuration"}
		analysis.Prevention = []string{"Test deployments in staging environment", "Add comprehensive health checks"}
		return analysis
	}

	// Node scheduling and taint patterns
	if strings.Contains(errorLower, "nodes are available") && strings.Contains(errorLower, "taint") {
		analysis.RootCause = "Kubernetes node has taints that prevent pod scheduling"
		analysis.FixSteps = []string{
			"Wait for node to become ready",
			"Remove node taints if safe to do so",
			"Add toleration to pod specification if needed",
		}
		analysis.Fix = "Resolve node scheduling constraints"
		analysis.Alternatives = []string{"Use different node selector", "Add node affinity rules"}
		analysis.Prevention = []string{"Monitor node health", "Configure proper cluster resources"}
		return analysis
	}

	// Image pull patterns
	if strings.Contains(errorLower, "image pull") || strings.Contains(errorLower, "pulling image") {
		analysis.RootCause = "Image pull operation in progress or failed"
		analysis.FixSteps = []string{
			"Wait for image pull to complete",
			"Verify image exists in registry",
			"Check network connectivity to registry",
			"Ensure proper image pull policy",
		}
		analysis.Fix = "Resolve image pull issues"
		analysis.Alternatives = []string{"Use local image", "Change image pull policy"}
		analysis.Prevention = []string{"Pre-pull images", "Use local registry", "Verify image availability"}
		return analysis
	}

	// Port configuration patterns
	if strings.Contains(errorLower, "port: 0") || (strings.Contains(errorLower, "port") && strings.Contains(errorLower, "connection")) {
		analysis.RootCause = "Application port is not properly configured or accessible"
		analysis.FixSteps = []string{
			"Set proper port in application configuration",
			"Add EXPOSE directive to Dockerfile",
			"Update Kubernetes service port configuration",
			"Verify application is listening on the correct port",
		}
		analysis.Fix = "Configure application port properly"
		analysis.Alternatives = []string{"Use different port number", "Configure port binding"}
		analysis.Prevention = []string{"Always specify ports explicitly", "Test port connectivity"}
		return analysis
	}

	// Docker build patterns
	if strings.Contains(errorLower, "dockerfile") && (strings.Contains(errorLower, "syntax") || strings.Contains(errorLower, "instruction")) {
		analysis.RootCause = "Dockerfile contains syntax errors or invalid instructions"
		analysis.FixSteps = []string{
			"Review Dockerfile syntax",
			"Check instruction names and format",
			"Verify base image name and tag",
			"Ensure proper FROM instruction",
		}
		analysis.Fix = "Fix Dockerfile syntax errors"
		analysis.Alternatives = []string{"Use different base image", "Simplify Dockerfile"}
		analysis.Prevention = []string{"Use Dockerfile linting", "Test Dockerfiles incrementally"}
		return analysis
	}

	// Generic fallback analysis
	analysis.RootCause = "Operation failed with unspecified error"
	analysis.FixSteps = []string{
		"Review error details carefully",
		"Check system prerequisites and dependencies",
		"Verify configuration and permissions",
		"Retry operation with corrected parameters",
	}
	analysis.Fix = "Investigate and resolve the underlying issue"
	analysis.Alternatives = []string{"Try alternative approaches", "Simplify the operation"}
	analysis.Prevention = []string{"Add better error handling", "Improve system monitoring"}
	analysis.CanAutoFix = false // Generic errors usually need manual intervention

	return analysis
}

func (c *Client) SetTokenBudget(budget int) {
	c.tokenBudget = budget
}

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

// applyAICorrection uses AI analysis to improve a failed sampling request
func (c *Client) applyAICorrection(ctx context.Context, originalReq SamplingRequest, errorHistory []string, attempt int) (SamplingRequest, error) {
	// Create a correction prompt that analyzes the errors and suggests improvements
	correctionPrompt := fmt.Sprintf(`You are an AI assistant helping to fix a failed LLM sampling request. 

ORIGINAL REQUEST:
- Prompt: %s
- Max Tokens: %d
- Temperature: %f
- System Prompt: %s

ERROR HISTORY (Attempts 1-%d):
%s

Your task: Analyze the errors and provide an improved version of the original prompt that addresses the issues. Focus on:
1. Clarity and specificity improvements
2. Better constraint specification
3. Format corrections
4. Context improvements

Respond with only the improved prompt text - no explanations or commentary.`,
		originalReq.Prompt,
		originalReq.MaxTokens,
		originalReq.Temperature,
		originalReq.SystemPrompt,
		attempt,
		strings.Join(errorHistory, "\n- "))

	correctionReq := SamplingRequest{
		Prompt:       correctionPrompt,
		MaxTokens:    1000, // Smaller token limit for corrections
		Temperature:  0.2,  // Lower temperature for more focused corrections
		SystemPrompt: "You are a helpful AI assistant that improves prompts for better LLM responses.",
	}

	c.logger.Debug("Requesting AI correction",
		"attempt", attempt,
		"error_count", len(errorHistory),
		"original_prompt_length", len(originalReq.Prompt))

	// Use direct MCP call to avoid recursion
	correctionResp, err := c.callMCP(ctx, correctionReq)
	if err != nil {
		return originalReq, errors.New(errors.CodeOperationFailed, "sampling",
			"AI correction request failed", err)
	}

	// Create improved request with the corrected prompt
	improvedReq := originalReq
	improvedReq.Prompt = strings.TrimSpace(correctionResp.Content)

	// If the correction seems valid (not empty and different), use it
	if len(improvedReq.Prompt) > 0 && improvedReq.Prompt != originalReq.Prompt {
		c.logger.Info("AI correction applied successfully",
			"attempt", attempt,
			"original_length", len(originalReq.Prompt),
			"corrected_length", len(improvedReq.Prompt))
		return improvedReq, nil
	}

	// If correction failed or was identical, return original
	c.logger.Warn("AI correction was empty or identical, keeping original", "attempt", attempt)
	return originalReq, fmt.Errorf("AI correction produced no improvement")
}

// Configuration types and functions (consolidated from config.go)

// Config holds configuration for the sampling client
type Config struct {
	MaxTokens        int32         `json:"max_tokens" env:"SAMPLING_MAX_TOKENS"`
	Temperature      float32       `json:"temperature" env:"SAMPLING_TEMPERATURE"`
	RetryAttempts    int           `json:"retry_attempts" env:"SAMPLING_RETRY_ATTEMPTS"`
	TokenBudget      int           `json:"token_budget" env:"SAMPLING_TOKEN_BUDGET"`
	BaseBackoff      time.Duration `json:"base_backoff" env:"SAMPLING_BASE_BACKOFF"`
	MaxBackoff       time.Duration `json:"max_backoff" env:"SAMPLING_MAX_BACKOFF"`
	StreamingEnabled bool          `json:"streaming_enabled" env:"SAMPLING_STREAMING_ENABLED"`
	RequestTimeout   time.Duration `json:"request_timeout" env:"SAMPLING_REQUEST_TIMEOUT"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		MaxTokens:        2048,
		Temperature:      0.3,
		RetryAttempts:    3,
		TokenBudget:      5000,
		BaseBackoff:      200 * time.Millisecond,
		MaxBackoff:       10 * time.Second,
		StreamingEnabled: false,
		RequestTimeout:   30 * time.Second,
	}
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() Config {
	cfg := DefaultConfig()

	if val := os.Getenv("SAMPLING_MAX_TOKENS"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 32); err == nil {
			cfg.MaxTokens = int32(parsed)
		}
	}

	if val := os.Getenv("SAMPLING_TEMPERATURE"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 32); err == nil {
			cfg.Temperature = float32(parsed)
		}
	}

	if val := os.Getenv("SAMPLING_RETRY_ATTEMPTS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			cfg.RetryAttempts = parsed
		}
	}

	if val := os.Getenv("SAMPLING_TOKEN_BUDGET"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			cfg.TokenBudget = parsed
		}
	}

	if val := os.Getenv("SAMPLING_BASE_BACKOFF"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			cfg.BaseBackoff = parsed
		}
	}

	if val := os.Getenv("SAMPLING_MAX_BACKOFF"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			cfg.MaxBackoff = parsed
		}
	}

	if val := os.Getenv("SAMPLING_STREAMING_ENABLED"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			cfg.StreamingEnabled = parsed
		}
	}

	if val := os.Getenv("SAMPLING_REQUEST_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			cfg.RequestTimeout = parsed
		}
	}

	return cfg
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	if c.MaxTokens <= 0 {
		return ErrInvalidConfig("max_tokens must be positive")
	}

	if c.Temperature < 0 || c.Temperature > 2 {
		return ErrInvalidConfig("temperature must be between 0 and 2")
	}

	if c.RetryAttempts < 0 {
		return ErrInvalidConfig("retry_attempts must be non-negative")
	}

	if c.TokenBudget <= 0 {
		return ErrInvalidConfig("token_budget must be positive")
	}

	if c.BaseBackoff <= 0 {
		return ErrInvalidConfig("base_backoff must be positive")
	}

	if c.MaxBackoff <= c.BaseBackoff {
		return ErrInvalidConfig("max_backoff must be greater than base_backoff")
	}

	if c.RequestTimeout <= 0 {
		return ErrInvalidConfig("request_timeout must be positive")
	}

	return nil
}

// WithConfig returns an Option that applies the given configuration
func WithConfig(cfg Config) Option {
	return func(c *Client) {
		c.maxTokens = cfg.MaxTokens
		c.temperature = cfg.Temperature
		c.retryAttempts = cfg.RetryAttempts
		c.tokenBudget = cfg.TokenBudget
		c.baseBackoff = cfg.BaseBackoff
		c.maxBackoff = cfg.MaxBackoff
		c.streamingEnabled = cfg.StreamingEnabled
		c.requestTimeout = cfg.RequestTimeout
	}
}

// ErrInvalidConfig represents a configuration error
type ErrInvalidConfig string

func (e ErrInvalidConfig) Error() string {
	return "invalid sampling config: " + string(e)
}

// Helper functions (consolidated from helpers.go)

// GetWorkflowIDFromContext extracts workflow ID from context with multiple fallbacks
func GetWorkflowIDFromContext(ctx context.Context) string {
	// Try common context keys for workflow ID
	keys := []interface{}{
		"workflow_id", "workflowID", "workflow",
		"session_id", "sessionID", "session",
		"request_id", "requestID", "request",
	}
	for _, key := range keys {
		if val := ctx.Value(key); val != nil {
			if id, ok := val.(string); ok && id != "" {
				return id
			}
		}
	}

	// Generate a fallback ID based on context if available
	if span := ctx.Value("span"); span != nil {
		return "ctx-derived"
	}

	return "unknown"
}

// GetStepNameFromContext extracts step name from context with multiple fallbacks
func GetStepNameFromContext(ctx context.Context) string {
	// Try common context keys for step name
	keys := []interface{}{
		"step_name", "stepName", "step", "current_step",
		"operation", "action", "task",
	}
	for _, key := range keys {
		if val := ctx.Value(key); val != nil {
			if step, ok := val.(string); ok && step != "" {
				return step
			}
		}
	}
	return "unknown"
}

// EstimateTokenCount provides a conservative token count estimate
func EstimateTokenCount(text string) int {
	// More sophisticated estimation considering:
	// - Word count (1.3 tokens per word on average)
	// - Character count (4 characters per token on average)
	// - Unicode considerations

	charCount := utf8.RuneCountInString(text)
	words := len(splitWords(text))

	// Use the more conservative estimate
	fromChars := charCount / 4
	fromWords := int(float64(words) * 1.3)

	if fromWords > fromChars {
		return fromWords
	}
	return fromChars
}

// splitWords splits text into words for more accurate token estimation
func splitWords(text string) []string {
	if text == "" {
		return nil
	}

	words := make([]string, 0)
	word := make([]rune, 0)

	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if len(word) > 0 {
				words = append(words, string(word))
				word = word[:0]
			}
		} else {
			word = append(word, r)
		}
	}

	if len(word) > 0 {
		words = append(words, string(word))
	}

	return words
}

// truncateText truncates a string to a maximum length with ellipsis
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// Contains checks if a string contains a substring (case-insensitive helper)
func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// IsRetryable determines if an error is retryable based on common patterns
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable error patterns
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "temporarily") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dns") ||
		strings.Contains(errStr, "no mcp server in context") // Allow retrying MCP server context issues
}
