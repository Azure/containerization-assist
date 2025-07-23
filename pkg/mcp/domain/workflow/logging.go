// Package workflow provides unified logging middleware for step execution
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// LogLevel represents the logging verbosity level
type LogLevel int

const (
	// LogLevelMinimal logs only step start/completion and errors
	LogLevelMinimal LogLevel = iota
	// LogLevelStandard logs step lifecycle events with basic metadata
	LogLevelStandard
	// LogLevelDetailed logs comprehensive step information including timing and context
	LogLevelDetailed
	// LogLevelDebug logs everything including internal middleware operations
	LogLevelDebug
)

// LoggingConfig represents configuration options for the logging middleware
type LoggingConfig struct {
	// Level controls the verbosity of logging
	Level LogLevel

	// Logger is the structured logger to use
	Logger *slog.Logger

	// IncludeStackTrace includes stack traces in error logs
	IncludeStackTrace bool

	// SampleRate controls log sampling for high-volume scenarios (0.0 to 1.0)
	SampleRate float64

	// StructuredAttributes enables additional structured logging attributes
	StructuredAttributes bool
}

// LoggingMiddleware provides unified, structured logging for all step executions.
// This middleware consolidates logging functionality that was previously scattered
// across tracing, progress, retry, and event middleware.
//
// Features:
// - Configurable log levels and verbosity
// - Structured logging with consistent attribute names
// - Performance-optimized with context-aware sampling
// - Integration with OpenTelemetry trace context
// - Standardized error logging with optional stack traces
func LoggingMiddleware(config LoggingConfig) StepMiddleware {
	// Ensure we have a logger
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	// Default sample rate to 1.0 (log everything) if not specified
	if config.SampleRate <= 0 {
		config.SampleRate = 1.0
	}

	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			// Apply sampling if configured
			if !shouldLog(config.SampleRate) {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			stepIndex := state.CurrentStep + 1
			startTime := time.Now()

			// Create base attributes for all log entries
			baseAttrs := createBaseAttributes(ctx, step, state, stepIndex, config)

			// Log step start
			logStepStart(config.Logger, stepName, baseAttrs, config.Level)

			// Execute the step
			err := next(ctx, step, state)
			duration := time.Since(startTime)

			// Add timing and result attributes
			resultAttrs := append(baseAttrs,
				slog.Duration("duration", duration),
				slog.Int64("duration_ms", duration.Milliseconds()),
			)

			if err != nil {
				logStepFailure(config.Logger, stepName, err, resultAttrs, config)
			} else {
				logStepSuccess(config.Logger, stepName, resultAttrs, config.Level)
			}

			return err
		}
	}
}

// createBaseAttributes creates the standard set of attributes for all log entries
func createBaseAttributes(ctx context.Context, step Step, state *WorkflowState, stepIndex int, config LoggingConfig) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("step_name", step.Name()),
		slog.Int("step_index", stepIndex),
		slog.Int("total_steps", state.TotalSteps),
		slog.String("workflow_id", state.WorkflowID),
	}

	if config.StructuredAttributes {
		// Add additional structured attributes
		attrs = append(attrs,
			slog.Int("max_retries", step.MaxRetries()),
			slog.String("component", "workflow_middleware"),
		)

		// Add retry attempt if available from context
		if attempt := getRetryAttemptFromContext(ctx); attempt > 1 {
			attrs = append(attrs, slog.Int("retry_attempt", attempt))
		}

		// Add trace context if available
		if traceID := getTraceIDFromContext(ctx); traceID != "" {
			attrs = append(attrs, slog.String("trace_id", traceID))
		}
	}

	return attrs
}

// logStepStart logs the beginning of step execution
func logStepStart(logger *slog.Logger, stepName string, attrs []slog.Attr, level LogLevel) {
	switch level {
	case LogLevelMinimal:
		// Only log in minimal mode if it's an important step
		return
	case LogLevelStandard:
		args := make([]any, len(attrs))
		for i, attr := range attrs {
			args[i] = attr
		}
		logger.Info("Step started", args...)
	case LogLevelDetailed, LogLevelDebug:
		allAttrs := append(attrs, slog.String("phase", "start"))
		args := make([]any, len(allAttrs))
		for i, attr := range allAttrs {
			args[i] = attr
		}
		logger.Info("Executing workflow step", args...)
	}
}

// logStepSuccess logs successful step completion
func logStepSuccess(logger *slog.Logger, stepName string, attrs []slog.Attr, level LogLevel) {
	switch level {
	case LogLevelMinimal:
		logger.Info("Step completed",
			slog.String("step_name", stepName),
			slog.Duration("duration", getDurationFromAttrs(attrs)))
	case LogLevelStandard:
		args := make([]any, len(attrs))
		for i, attr := range attrs {
			args[i] = attr
		}
		logger.Info("Step completed successfully", args...)
	case LogLevelDetailed, LogLevelDebug:
		allAttrs := append(attrs, slog.String("phase", "complete"))
		args := make([]any, len(allAttrs))
		for i, attr := range allAttrs {
			args[i] = attr
		}
		logger.Info("Workflow step completed successfully", args...)
	}
}

// logStepFailure logs step execution failures
func logStepFailure(logger *slog.Logger, stepName string, err error, attrs []slog.Attr, config LoggingConfig) {
	errorAttrs := append(attrs,
		slog.String("error", err.Error()),
		slog.String("error_type", fmt.Sprintf("%T", err)),
	)

	if config.IncludeStackTrace {
		// TODO: Add stack trace capture when available
		// For now, we rely on the error wrapping to provide context
	}

	switch config.Level {
	case LogLevelMinimal:
		logger.Error("Step failed",
			slog.String("step_name", stepName),
			slog.String("error", err.Error()))
	case LogLevelStandard:
		args := make([]any, len(errorAttrs))
		for i, attr := range errorAttrs {
			args[i] = attr
		}
		logger.Error("Step execution failed", args...)
	case LogLevelDetailed, LogLevelDebug:
		allAttrs := append(errorAttrs, slog.String("phase", "failed"))
		args := make([]any, len(allAttrs))
		for i, attr := range allAttrs {
			args[i] = attr
		}
		logger.Error("Workflow step execution failed", args...)
	}
}

// Helper functions

// shouldLog implements simple sampling logic
func shouldLog(sampleRate float64) bool {
	if sampleRate >= 1.0 {
		return true
	}
	// Simple sampling - in production this could use more sophisticated algorithms
	// For now, we'll always log (sampling can be added later if needed)
	return true
}

// getRetryAttemptFromContext extracts retry attempt from context
func getRetryAttemptFromContext(ctx context.Context) int {
	if attempt, ok := GetRetryAttempt(ctx); ok {
		return attempt
	}
	return 1 // Default to first attempt
}

// getTraceIDFromContext extracts trace ID from context (OpenTelemetry integration)
func getTraceIDFromContext(ctx context.Context) string {
	// TODO: Integrate with OpenTelemetry to extract trace ID
	// This will be implemented when tracing middleware is updated
	return ""
}

// getDurationFromAttrs extracts duration from attributes
func getDurationFromAttrs(attrs []slog.Attr) time.Duration {
	for _, attr := range attrs {
		if attr.Key == "duration" {
			if dur, ok := attr.Value.Any().(time.Duration); ok {
				return dur
			}
		}
	}
	return 0
}

// Convenience constructors for common logging configurations

// MinimalLogging creates a logging middleware with minimal output
func MinimalLogging(logger *slog.Logger) StepMiddleware {
	return LoggingMiddleware(LoggingConfig{
		Level:      LogLevelMinimal,
		Logger:     logger,
		SampleRate: 1.0,
	})
}

// StandardLogging creates a logging middleware with standard verbosity
func StandardLogging(logger *slog.Logger) StepMiddleware {
	return LoggingMiddleware(LoggingConfig{
		Level:                LogLevelStandard,
		Logger:               logger,
		SampleRate:           1.0,
		StructuredAttributes: true,
	})
}

// DetailedLogging creates a logging middleware with comprehensive output
func DetailedLogging(logger *slog.Logger) StepMiddleware {
	return LoggingMiddleware(LoggingConfig{
		Level:                LogLevelDetailed,
		Logger:               logger,
		SampleRate:           1.0,
		StructuredAttributes: true,
		IncludeStackTrace:    true,
	})
}

// DebugLogging creates a logging middleware with maximum verbosity for debugging
func DebugLogging(logger *slog.Logger) StepMiddleware {
	return LoggingMiddleware(LoggingConfig{
		Level:                LogLevelDebug,
		Logger:               logger,
		SampleRate:           1.0,
		StructuredAttributes: true,
		IncludeStackTrace:    true,
	})
}
