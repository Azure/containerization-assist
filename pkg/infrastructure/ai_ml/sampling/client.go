package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/domain/sampling"
	"github.com/Azure/containerization-assist/pkg/infrastructure/ai_ml/prompts"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Option func(*Client)

// Client delegates LLM work to the calling AI assistant via the MCP sampling API.
type Client struct {
	logger        *slog.Logger
	maxTokens     int32
	temperature   float32
	retryAttempts int
	baseBackoff   time.Duration
	maxBackoff    time.Duration
}

func NewClient(logger *slog.Logger, opts ...Option) *Client {
	cfg := DefaultConfig()
	c := &Client{
		logger:        logger,
		maxTokens:     cfg.MaxTokens,
		temperature:   cfg.Temperature,
		retryAttempts: cfg.RetryAttempts,
		baseBackoff:   cfg.BaseBackoff,
		maxBackoff:    cfg.MaxBackoff,
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

		// Add retry attempt to span
		reqLogger.Debug("Sampling retry attempt", "attempt", attempt+1)

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

			// Log success attributes
			return resp, nil
		}

		// Log error with enhanced context
		enhancedLogger.LogLLMError(ctx, reqLogger, currentReq, err, time.Since(start), attempt+1)

		// Abort early on non-retryable errors.
		if !IsRetryable(err) {
			reqLogger.Error("Non-retryable sampling error", "error", err.Error())
			return nil, err
		}

		lastErr = err
		errorHistory = append(errorHistory, err.Error())

		// If this isn't the last attempt, try to use AI to improve the request
		if attempt < c.retryAttempts-1 {

			improvedReq, correctionErr := c.applyAICorrection(ctx, currentReq, errorHistory, attempt+1)
			if correctionErr != nil {
				// Continue with current request
			} else {
				currentReq = improvedReq
			}
		}

		backoff := c.calculateBackoff(attempt)

		// Log retry backoff
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
	reqLogger.Error("Sampling failed after all retries", "error", finalErr.Error())
	return nil, finalErr
}

func (c *Client) callMCP(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	// Try to get MCP server from context
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		// Return proper structured error
		return nil, errors.New(errors.CodeDisabled, "sampling", "MCP server not found in context", nil)
	}

	// Use actual MCP sampling when server is available
	// This enables AI-powered error analysis during deployment failures
	return c.callMCPSampling(ctx, srv, req)
}

// callMCPSampling performs actual MCP sampling using the server's sampling API
func (c *Client) callMCPSampling(ctx context.Context, srv *server.MCPServer, req SamplingRequest) (*SamplingResponse, error) {

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
	}

	if req.FrequencyPenalty != nil {
	}

	if req.PresencePenalty != nil {
	}

	if len(req.StopSequences) > 0 {
		// Many AI models support stop sequences, but we need to check if MCP supports them
	}

	if req.Seed != nil {
	}

	if len(req.LogitBias) > 0 {
	}

	// Make the actual MCP sampling request
	result, err := srv.RequestSampling(ctx, samplingRequest)
	if err != nil {
		return nil, errors.New(errors.CodeOperationFailed, "sampling", fmt.Sprintf("MCP sampling failed: %v", err), err)
	}

	// Extract content from the result (CreateMessageResult embeds SamplingMessage)
	var content string
	var tokensUsed int
	var model string

	if result.Content != nil {
		// Try to extract as TextContent first
		if textContent, ok := result.Content.(mcp.TextContent); ok {
			content = textContent.Text
		} else if contentMap, ok := result.Content.(map[string]interface{}); ok {
			// Handle map[string]interface{} format with "text" key
			if textValue, exists := contentMap["text"]; exists {
				if textStr, ok := textValue.(string); ok {
					content = textStr
				}
			}
		} else {
		}
	}

	// Log if we got empty content from MCP
	if content == "" {
	}

	// Estimate token usage since MCP doesn't provide usage statistics
	tokensUsed = estimateTokens(content)

	if result.Model != "" {
		model = result.Model
	} else {
		model = "mcp-ai-assistant"
	}

	return &SamplingResponse{
		Content:    content,
		TokensUsed: tokensUsed,
		Model:      model,
		StopReason: result.StopReason,
	}, nil
}

// estimateTokens provides a rough token count estimate.
// Uses empirical multiplier of 1.3 tokens per word.
func estimateTokens(s string) int {
	words := len(strings.Fields(s))
	return int(float64(words) * 1.3)
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
		return c.createPatternBasedErrorAnalysis(inputErr.Error(), contextInfo), nil
	}

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
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
		return c.createPatternBasedErrorAnalysis(inputErr.Error(), contextInfo), nil
	}

	// Parse AI response, but fallback to pattern-based if parsing fails
	analysis := parseErrorAnalysis(response.Content)
	if analysis.RootCause == "" && len(analysis.FixSteps) == 0 {
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

	// Use direct MCP call to avoid recursion
	correctionResp, err := c.callMCP(ctx, correctionReq)
	if err != nil {
		return originalReq, errors.New(errors.CodeOperationFailed, "sampling", fmt.Sprintf("failed to apply AI correction: %v", err), err)
	}

	// Create improved request with the corrected prompt
	improvedReq := originalReq
	improvedReq.Prompt = strings.TrimSpace(correctionResp.Content)

	// If the correction seems valid (not empty and different), use it
	if len(improvedReq.Prompt) > 0 && improvedReq.Prompt != originalReq.Prompt {
		return improvedReq, nil
	}

	// If correction failed or was identical, return original
	return originalReq, fmt.Errorf("AI correction produced no improvement")
}

// Configuration types and functions (consolidated from config.go)

// Config holds configuration for the sampling client
type Config struct {
	MaxTokens     int32         `json:"max_tokens" env:"SAMPLING_MAX_TOKENS"`
	Temperature   float32       `json:"temperature" env:"SAMPLING_TEMPERATURE"`
	RetryAttempts int           `json:"retry_attempts" env:"SAMPLING_RETRY_ATTEMPTS"`
	BaseBackoff   time.Duration `json:"base_backoff" env:"SAMPLING_BASE_BACKOFF"`
	MaxBackoff    time.Duration `json:"max_backoff" env:"SAMPLING_MAX_BACKOFF"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		MaxTokens:     2048,
		Temperature:   0.3,
		RetryAttempts: 3,
		BaseBackoff:   200 * time.Millisecond,
		MaxBackoff:    10 * time.Second,
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

	if c.BaseBackoff <= 0 {
		return ErrInvalidConfig("base_backoff must be positive")
	}

	if c.MaxBackoff <= c.BaseBackoff {
		return ErrInvalidConfig("max_backoff must be greater than base_backoff")
	}

	return nil
}

// WithConfig returns an Option that applies the given configuration
func WithConfig(cfg Config) Option {
	return func(c *Client) {
		c.maxTokens = cfg.MaxTokens
		c.temperature = cfg.Temperature
		c.retryAttempts = cfg.RetryAttempts
		c.baseBackoff = cfg.BaseBackoff
		c.maxBackoff = cfg.MaxBackoff
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
	keys := []interface{}{}
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
	keys := []interface{}{}
	for _, key := range keys {
		if val := ctx.Value(key); val != nil {
			if step, ok := val.(string); ok && step != "" {
				return step
			}
		}
	}
	return "unknown"
}

// truncateText truncates a string to a maximum length with ellipsis
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
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

// ============================================================================
// Domain Interface Implementation - UnifiedSampler
// ============================================================================

// Ensure Client implements the domain interface
var _ sampling.UnifiedSampler = (*Client)(nil)

// Sample implements the domain UnifiedSampler interface
func (c *Client) Sample(ctx context.Context, req sampling.Request) (sampling.Response, error) {
	// Convert domain request to internal request
	internalReq := SamplingRequest{
		Prompt:       req.Prompt,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		SystemPrompt: req.SystemPrompt,
	}

	resp, err := c.SampleInternal(ctx, internalReq)
	if err != nil {
		return sampling.Response{}, err
	}

	return sampling.Response{
		Content:    resp.Content,
		TokensUsed: resp.TokensUsed,
	}, nil
}

// Stream implements the domain UnifiedSampler interface
func (c *Client) Stream(ctx context.Context, req sampling.Request) (<-chan sampling.StreamChunk, error) {
	// For now, return an error as streaming is not implemented in the main client
	// This can be enhanced later if streaming is needed
	return nil, fmt.Errorf("streaming not implemented")
}

// AnalyzeDockerfile implements the domain UnifiedSampler interface
func (c *Client) AnalyzeDockerfile(ctx context.Context, content string) (*sampling.DockerfileAnalysis, error) {
	// Use internal sampling to analyze dockerfile
	req := SamplingRequest{
		Prompt:    fmt.Sprintf("Analyze this Dockerfile for best practices and issues:\n\n%s", content),
		MaxTokens: c.maxTokens,
	}

	resp, err := c.SampleInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	// For now, use simple defaults - could be enhanced to parse from resp.Content
	return &sampling.DockerfileAnalysis{
		Language:     "detected",             // Could be parsed from resp.Content
		Framework:    "unknown",              // Could be parsed from resp.Content
		Port:         8080,                   // Default port
		BuildSteps:   []string{resp.Content}, // Use response content
		Dependencies: []string{},             // Parse from response if needed
	}, nil
}

// AnalyzeKubernetesManifest implements the domain UnifiedSampler interface
func (c *Client) AnalyzeKubernetesManifest(ctx context.Context, content string) (*sampling.ManifestAnalysis, error) {
	req := SamplingRequest{
		Prompt:    fmt.Sprintf("Analyze this Kubernetes manifest for best practices and issues:\n\n%s", content),
		MaxTokens: c.maxTokens,
	}

	resp, err := c.SampleInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	return &sampling.ManifestAnalysis{
		ResourceTypes: []string{"Deployment", "Service"}, // Parse from response if needed
		Issues:        []string{},                        // Parse from response content if needed
		Suggestions:   []string{resp.Content},            // Use response content
		SecurityRisks: []string{},                        // Parse from response if needed
		BestPractices: []string{},                        // Parse from response if needed
	}, nil
}

// AnalyzeSecurityScan implements the domain UnifiedSampler interface
func (c *Client) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*sampling.SecurityAnalysis, error) {
	req := SamplingRequest{
		Prompt:    fmt.Sprintf("Analyze these security scan results and provide recommendations:\n\n%s", scanResults),
		MaxTokens: c.maxTokens,
	}

	resp, err := c.SampleInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	return &sampling.SecurityAnalysis{
		RiskLevel:       "low",                      // Parse from response if needed
		Vulnerabilities: []sampling.Vulnerability{}, // Parse from response content if needed
		Recommendations: []string{resp.Content},     // Use response content
		Remediations:    []string{},                 // Parse from response if needed
	}, nil
}

// FixDockerfile implements the domain UnifiedSampler interface
func (c *Client) FixDockerfile(ctx context.Context, content string, issues []string) (*sampling.DockerfileFix, error) {
	issuesText := strings.Join(issues, "\n- ")
	req := SamplingRequest{
		Prompt:    fmt.Sprintf("Fix these issues in the Dockerfile:\n\nIssues:\n- %s\n\nDockerfile:\n%s", issuesText, content),
		MaxTokens: c.maxTokens,
	}

	resp, err := c.SampleInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	return &sampling.DockerfileFix{
		FixedContent: resp.Content, // The LLM should return the fixed dockerfile
		Changes:      []string{},   // Parse from response if needed
		Explanation:  resp.Content,
	}, nil
}

// FixKubernetesManifest implements the domain UnifiedSampler interface
func (c *Client) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*sampling.ManifestFix, error) {
	issuesText := strings.Join(issues, "\n- ")
	req := SamplingRequest{
		Prompt:    fmt.Sprintf("Fix these issues in the Kubernetes manifest:\n\nIssues:\n- %s\n\nManifest:\n%s", issuesText, content),
		MaxTokens: c.maxTokens,
	}

	resp, err := c.SampleInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	return &sampling.ManifestFix{
		FixedContent: resp.Content, // The LLM should return the fixed manifest
		Changes:      []string{},   // Parse from response if needed
		Explanation:  resp.Content,
	}, nil
}
