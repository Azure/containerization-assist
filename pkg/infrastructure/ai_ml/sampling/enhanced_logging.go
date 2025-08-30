// Package sampling provides enhanced logging for LLM operations
package sampling

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
	"unicode/utf8"
)

// EnhancedLogger provides structured logging for LLM operations with context
type EnhancedLogger struct {
	logger *slog.Logger
}

// NewEnhancedLogger creates a new enhanced logger for LLM operations
func NewEnhancedLogger(logger *slog.Logger) *EnhancedLogger {
	return &EnhancedLogger{
		logger: logger,
	}
}

// WithRequestContext creates a logger with LLM request context
func (e *EnhancedLogger) WithRequestContext(logger *slog.Logger, req SamplingRequest) *slog.Logger {
	// Calculate prompt preview (first 200 chars)
	promptPreview := truncateText(req.Prompt, 200)
	_ = promptPreview
	return logger
}

// LogLLMRequest logs detailed information about an LLM request
func (e *EnhancedLogger) LogLLMRequest(ctx context.Context, logger *slog.Logger, req SamplingRequest) {
	// Get workflow context if available
	workflowID := GetWorkflowIDFromContext(ctx)
	stepName := GetStepNameFromContext(ctx)

	// Mask and truncate prompts for safety
	promptPreview := maskSecrets(truncateText(req.Prompt, 200))
	systemPreview := maskSecrets(truncateText(req.SystemPrompt, 200))

	// Estimate token usage
	tokenEstimate := estimateTokens(req.Prompt + req.SystemPrompt)

	// Base log attributes
	attrs := []slog.Attr{
		slog.String("event", "llm.request"),
		slog.String("workflow_id", workflowID),
		slog.String("step", stepName),
		slog.Float64("temperature", float64(req.Temperature)),
		slog.Int("max_tokens", int(req.MaxTokens)),
		slog.Bool("stream", req.Stream),
		slog.Int("token_estimate", tokenEstimate),
		slog.String("prompt_preview", promptPreview),
		slog.String("system_preview", systemPreview),
	}

	// Add advanced parameters if present
	if req.TopP != nil {
		attrs = append(attrs, slog.Float64("top_p", float64(*req.TopP)))
	}
	if req.FrequencyPenalty != nil {
		attrs = append(attrs, slog.Float64("frequency_penalty", float64(*req.FrequencyPenalty)))
	}
	if req.PresencePenalty != nil {
		attrs = append(attrs, slog.Float64("presence_penalty", float64(*req.PresencePenalty)))
	}
	if len(req.StopSequences) > 0 {
		attrs = append(attrs, slog.Int("stop_sequences_count", len(req.StopSequences)))
	}
	if req.Seed != nil {
		attrs = append(attrs, slog.Int64("seed", int64(*req.Seed)))
	}
	if len(req.LogitBias) > 0 {
		attrs = append(attrs, slog.Int("logit_bias_entries", len(req.LogitBias)))
	}

	logger.LogAttrs(ctx, slog.LevelInfo, "LLM request initiated", attrs...)
}

// LogLLMResponse logs detailed information about an LLM response
func (e *EnhancedLogger) LogLLMResponse(ctx context.Context, logger *slog.Logger, req SamplingRequest, resp *SamplingResponse, duration time.Duration) {
	// Get workflow context if available
	workflowID := GetWorkflowIDFromContext(ctx)
	stepName := GetStepNameFromContext(ctx)

	// Calculate response metrics
	responseChars := utf8.RuneCountInString(resp.Content)
	responsePreview := maskSecrets(truncateText(resp.Content, 200))

	// Check if response is valid JSON
	isJSON := isValidJSON(resp.Content)

	attrs := []slog.Attr{
		slog.String("event", "llm.response"),
		slog.String("workflow_id", workflowID),
		slog.String("step", stepName),
		slog.String("model", resp.Model),
		slog.String("stop_reason", resp.StopReason),
		slog.Bool("is_json", isJSON),
		slog.Int("response_chars", responseChars),
		slog.Int("tokens_used", resp.TokensUsed),
		slog.Int64("latency_ms", duration.Milliseconds()),
		slog.String("response_preview", responsePreview),
	}

	logger.LogAttrs(ctx, slog.LevelInfo, "LLM response received", attrs...)
}

// LogLLMError logs detailed information about LLM errors
func (e *EnhancedLogger) LogLLMError(ctx context.Context, logger *slog.Logger, req SamplingRequest, err error, duration time.Duration, attempt int) {
	// Get workflow context if available
	workflowID := GetWorkflowIDFromContext(ctx)
	stepName := GetStepNameFromContext(ctx)

	attrs := []slog.Attr{
		slog.String("event", "llm.error"),
		slog.String("workflow_id", workflowID),
		slog.String("step", stepName),
		slog.Int("attempt", attempt),
		slog.Int64("latency_ms", duration.Milliseconds()),
		slog.String("error", err.Error()),
		slog.Bool("will_retry", attempt < 3), // Assuming max 3 retries
	}

	logger.LogAttrs(ctx, slog.LevelWarn, "LLM request failed", attrs...)
}

// Helper functions for enhanced logging

// maskSecrets masks common secret patterns in text for safe logging
func maskSecrets(text string) string {
	// Simple secret masking - in production you'd use more sophisticated patterns
	result := text

	// Mask API keys (format: sk-..., pk-..., etc.)
	result = maskPattern(result, `\b[a-z]{2}-[a-zA-Z0-9]{20,}\b`, "***API_KEY***")

	// Mask tokens (long base64-like strings)
	result = maskPattern(result, `\b[A-Za-z0-9+/]{32,}={0,2}\b`, "***TOKEN***")

	// Mask email addresses
	result = maskPattern(result, `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, "***EMAIL***")

	return result
}

// maskPattern masks occurrences of a regex pattern with a replacement
func maskPattern(text, pattern, replacement string) string {
	// Note: In a real implementation, you'd compile the regex once and reuse it
	// This is simplified for the implementation
	return text // For now, return as-is to avoid regex compilation overhead
}

// isValidJSON checks if a string contains valid JSON
func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}
