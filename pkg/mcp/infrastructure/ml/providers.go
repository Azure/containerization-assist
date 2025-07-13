// Package ml provides Wire providers for machine learning components.
package ml

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// ProvideErrorPatternRecognizer creates an error pattern recognizer
func ProvideErrorPatternRecognizer(samplingClient *sampling.Client, logger *slog.Logger) *ErrorPatternRecognizer {
	return NewErrorPatternRecognizer(samplingClient, logger)
}

// ProvideEnhancedErrorHandler creates an enhanced error handler
func ProvideEnhancedErrorHandler(
	samplingClient *sampling.Client,
	eventPublisher *events.Publisher,
	logger *slog.Logger,
) *EnhancedErrorHandler {
	return NewEnhancedErrorHandler(samplingClient, eventPublisher, logger)
}

// ProvideStepEnhancer creates a step enhancer for AI-powered workflow steps
func ProvideStepEnhancer(errorHandler *EnhancedErrorHandler, logger *slog.Logger) *StepEnhancer {
	return NewStepEnhancer(errorHandler, logger)
}
