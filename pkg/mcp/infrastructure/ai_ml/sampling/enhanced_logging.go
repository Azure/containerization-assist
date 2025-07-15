// Package sampling provides enhanced logging for LLM operations
package sampling

import (
	"context"
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

// WithWorkflowContext creates a logger with workflow context information
func (e *EnhancedLogger) WithWorkflowContext(workflowID, stepName string) *slog.Logger {
	return e.logger.With(
		"workflow_id", workflowID,
		"step", stepName,
		"component", "llm-sampling",
	)
}

// WithRequestContext creates a logger with LLM request context
func (e *EnhancedLogger) WithRequestContext(logger *slog.Logger, req SamplingRequest) *slog.Logger {
	// Calculate prompt preview (first 200 chars)
	promptPreview := truncateText(req.Prompt, 200)

	return logger.With(
		"request_id", generateRequestID(),
		"prompt_length", len(req.Prompt),
		"prompt_chars", utf8.RuneCountInString(req.Prompt),
		"max_tokens", req.MaxTokens,
		"temperature", req.Temperature,
		"has_system_prompt", req.SystemPrompt != "",
		"system_prompt_length", len(req.SystemPrompt),
		"prompt_preview", promptPreview,
	)
}

// LogLLMRequest logs detailed information about an LLM request
func (e *EnhancedLogger) LogLLMRequest(ctx context.Context, logger *slog.Logger, req SamplingRequest) {
	// Get workflow context if available
	workflowID := GetWorkflowIDFromContext(ctx)
	stepName := GetStepNameFromContext(ctx)

	// Enhanced request logging with structured context
	logger.Debug("LLM request initiated",
		"workflow_id", workflowID,
		"step", stepName,
		"messages_count", 1, // Always 1 for current impl
		"prompt_length", len(req.Prompt),
		"prompt_chars", utf8.RuneCountInString(req.Prompt),
		"max_tokens", req.MaxTokens,
		"temperature", req.Temperature,
		"has_system_prompt", req.SystemPrompt != "",
		"system_prompt_length", len(req.SystemPrompt),
		"prompt_preview", truncateText(req.Prompt, 200),
		"request_timestamp", time.Now().Format(time.RFC3339),
	)

	// Log advanced parameters if present
	if req.TopP != nil {
		logger.Debug("LLM advanced parameter", "param", "top_p", "value", *req.TopP)
	}
	if req.FrequencyPenalty != nil {
		logger.Debug("LLM advanced parameter", "param", "frequency_penalty", "value", *req.FrequencyPenalty)
	}
	if req.PresencePenalty != nil {
		logger.Debug("LLM advanced parameter", "param", "presence_penalty", "value", *req.PresencePenalty)
	}
	if len(req.StopSequences) > 0 {
		logger.Debug("LLM advanced parameter", "param", "stop_sequences", "count", len(req.StopSequences))
	}
	if req.Seed != nil {
		logger.Debug("LLM advanced parameter", "param", "seed", "value", *req.Seed)
	}
	if len(req.LogitBias) > 0 {
		logger.Debug("LLM advanced parameter", "param", "logit_bias", "entries", len(req.LogitBias))
	}
}

// LogLLMResponse logs detailed information about an LLM response
func (e *EnhancedLogger) LogLLMResponse(ctx context.Context, logger *slog.Logger, req SamplingRequest, resp *SamplingResponse, duration time.Duration) {
	// Get workflow context if available
	workflowID := GetWorkflowIDFromContext(ctx)
	stepName := GetStepNameFromContext(ctx)

	// Calculate response metrics
	responseChars := utf8.RuneCountInString(resp.Content)
	responsePreview := truncateText(resp.Content, 200)

	logger.Info("LLM response received",
		"workflow_id", workflowID,
		"step", stepName,
		"latency", duration,
		"prompt_tokens", EstimateTokenCount(req.Prompt),
		"completion_tokens", resp.TokensUsed,
		"total_tokens", EstimateTokenCount(req.Prompt)+resp.TokensUsed,
		"response_length", len(resp.Content),
		"response_chars", responseChars,
		"model", resp.Model,
		"stop_reason", resp.StopReason,
		"response_preview", responsePreview,
		"tokens_per_second", calculateTokensPerSecond(resp.TokensUsed, duration),
		"response_timestamp", time.Now().Format(time.RFC3339),
	)
}

// LogLLMError logs detailed information about LLM errors
func (e *EnhancedLogger) LogLLMError(ctx context.Context, logger *slog.Logger, req SamplingRequest, err error, duration time.Duration, attempt int) {
	// Get workflow context if available
	workflowID := GetWorkflowIDFromContext(ctx)
	stepName := GetStepNameFromContext(ctx)

	logger.Error("LLM request failed",
		"workflow_id", workflowID,
		"step", stepName,
		"error", err.Error(),
		"attempt", attempt,
		"duration", duration,
		"prompt_length", len(req.Prompt),
		"max_tokens", req.MaxTokens,
		"temperature", req.Temperature,
		"error_timestamp", time.Now().Format(time.RFC3339),
	)
}

// LogTemplateLoading logs template loading diagnostics
func (e *EnhancedLogger) LogTemplateLoading(logger *slog.Logger, templateName string, directories []string, err error) {
	if err != nil {
		logger.Error("Failed loading AI template",
			"template", templateName,
			"search_dirs", directories,
			"error", err.Error(),
			"directories_count", len(directories),
		)
	} else {
		logger.Debug("AI template loaded successfully",
			"template", templateName,
			"search_dirs", directories,
			"directories_count", len(directories),
		)
	}
}

// Helper functions

func generateRequestID() string {
	return time.Now().Format("20060102-150405.000")
}

func calculateTokensPerSecond(tokens int, duration time.Duration) float64 {
	if duration.Seconds() == 0 {
		return 0
	}
	return float64(tokens) / duration.Seconds()
}
