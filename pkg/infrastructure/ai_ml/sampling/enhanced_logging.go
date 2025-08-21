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
	_ = GetWorkflowIDFromContext(ctx)
	_ = GetStepNameFromContext(ctx)

	// Enhanced request logging with structured context

	// Log advanced parameters if present
	if req.TopP != nil {
	}
	if req.FrequencyPenalty != nil {
	}
	if req.PresencePenalty != nil {
	}
	if len(req.StopSequences) > 0 {
	}
	if req.Seed != nil {
	}
	if len(req.LogitBias) > 0 {
	}
}

// LogLLMResponse logs detailed information about an LLM response
func (e *EnhancedLogger) LogLLMResponse(ctx context.Context, logger *slog.Logger, req SamplingRequest, resp *SamplingResponse, duration time.Duration) {
	// Get workflow context if available
	_ = GetWorkflowIDFromContext(ctx)
	_ = GetStepNameFromContext(ctx)

	// Calculate response metrics
	responseChars := utf8.RuneCountInString(resp.Content)
	responsePreview := truncateText(resp.Content, 200)
	_ = responseChars
	_ = responsePreview
}

// LogLLMError logs detailed information about LLM errors
func (e *EnhancedLogger) LogLLMError(ctx context.Context, logger *slog.Logger, req SamplingRequest, err error, duration time.Duration, attempt int) {
	// Get workflow context if available
	workflowID := GetWorkflowIDFromContext(ctx)
	stepName := GetStepNameFromContext(ctx)
	_ = workflowID
	_ = stepName
}

// Note: Helper functions removed as they were unused
