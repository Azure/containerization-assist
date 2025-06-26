package observability

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

var (
	globalErrorMetrics     *ErrorMetrics
	globalErrorMetricsOnce sync.Once
	globalLogger           zerolog.Logger
)

// InitializeGlobalMetrics initializes the global error metrics instance
func InitializeGlobalMetrics(logger zerolog.Logger) {
	globalErrorMetricsOnce.Do(func() {
		globalErrorMetrics = NewErrorMetrics()
		globalLogger = logger.With().Str("component", "error_metrics").Logger()
		globalLogger.Info().Msg("Initialized global error metrics")
	})
}

// GetGlobalErrorMetrics returns the global error metrics instance
func GetGlobalErrorMetrics() *ErrorMetrics {
	if globalErrorMetrics == nil {
		// Initialize with a default logger if not already initialized
		InitializeGlobalMetrics(zerolog.Nop())
	}
	return globalErrorMetrics
}

// RecordRichError is a convenience function to record errors globally
func RecordRichError(ctx context.Context, err *types.RichError) {
	if err == nil {
		return
	}

	metrics := GetGlobalErrorMetrics()
	metrics.RecordError(ctx, err)

	// Log error details if logger is available
	if globalLogger.GetLevel() != zerolog.Disabled {
		globalLogger.Error().
			Str("code", err.Code).
			Str("type", err.Type).
			Str("severity", err.Severity).
			Str("component", err.Context.Component).
			Str("operation", err.Context.Operation).
			Str("message", err.Message).
			Msg("Error recorded in metrics")
	}
}

// RecordErrorResolution is a convenience function to record error resolutions globally
func RecordErrorResolution(ctx context.Context, err *types.RichError, resolutionType string, duration time.Duration) {
	if err == nil {
		return
	}

	metrics := GetGlobalErrorMetrics()
	metrics.RecordResolution(ctx, err, resolutionType, duration)

	// Log resolution if logger is available
	if globalLogger.GetLevel() != zerolog.Disabled {
		globalLogger.Info().
			Str("code", err.Code).
			Str("type", err.Type).
			Str("resolution_type", resolutionType).
			Dur("duration", duration).
			Msg("Error resolution recorded")
	}
}

// EnrichErrorContext adds observability context to errors globally
func EnrichErrorContext(ctx context.Context, err *types.RichError) {
	if err == nil {
		return
	}

	metrics := GetGlobalErrorMetrics()
	metrics.EnrichContext(ctx, err)
}
