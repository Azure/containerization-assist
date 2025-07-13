// Package sampling provides factory methods for creating sampling clients with middleware
package sampling

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/middleware/metrics"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/middleware/retry"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/middleware/trace"
	"go.opentelemetry.io/otel"
)

// ClientOptions contains options for creating a sampling client
type ClientOptions struct {
	EnableTracing    bool
	EnableMetrics    bool
	EnableRetry      bool
	RetryConfig      *retry.Config
	MetricsCollector metrics.MetricsCollector
}

// DefaultClientOptions returns default client options
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		EnableTracing: true,
		EnableMetrics: true,
		EnableRetry:   true,
	}
}

// NewClientWithMiddleware creates a new sampling client with middleware pipeline
func NewClientWithMiddleware(logger *slog.Logger, opts ClientOptions) sampling.UnifiedSampler {
	// Start with core client
	var sampler sampling.UnifiedSampler = NewCoreClient(logger)

	// Apply retry middleware (innermost, closest to core)
	if opts.EnableRetry {
		retryConfig := retry.DefaultConfig()
		if opts.RetryConfig != nil {
			retryConfig = *opts.RetryConfig
		}
		sampler = retry.New(sampler, retryConfig, logger.With("middleware", "retry"))
	}

	// Apply metrics middleware
	if opts.EnableMetrics && opts.MetricsCollector != nil {
		sampler = metrics.New(sampler, opts.MetricsCollector)
	}

	// Apply tracing middleware (outermost)
	if opts.EnableTracing {
		tracer := otel.Tracer("container-kit/sampling")
		sampler = trace.New(sampler, tracer)
	}

	return sampler
}

// NewClientCompat creates a new sampling client for backward compatibility
// Deprecated: Use NewClientWithMiddleware or CreateDomainClient instead
func NewClientCompat(logger *slog.Logger) *Client {
	// Create the old client for backward compatibility
	return &Client{
		logger: logger,
	}
}

// CreateDomainClient creates a domain-compatible client with full middleware
func CreateDomainClient(logger *slog.Logger) sampling.UnifiedSampler {
	opts := DefaultClientOptions()

	// Use the global metrics collector if available
	metricsCollector := GetGlobalMetrics()
	if metricsCollector != nil {
		opts.MetricsCollector = &metricsAdapter{metrics: metricsCollector}
	}

	return NewClientWithMiddleware(logger, opts)
}

// metricsAdapter adapts the old metrics interface to the new MetricsCollector
type metricsAdapter struct {
	metrics interface {
		RecordSamplingRequest(ctx context.Context, operation string, success bool, duration time.Duration, contentSize, responseSize, tokensUsed int, contentType string, outputSize int, validation ValidationResult)
	}
}

func (m *metricsAdapter) RecordSamplingRequest(ctx context.Context, operation string, success bool, duration time.Duration, contentSize int, responseSize int, validation metrics.ValidationResult) {
	// Convert to old format
	oldValidation := ValidationResult{
		IsValid:       validation.IsValid,
		SyntaxValid:   validation.SyntaxValid,
		BestPractices: validation.BestPractices,
		Errors:        validation.Errors,
		Warnings:      validation.Warnings,
	}

	// Pass zeros for the missing parameters (tokensUsed, contentType, outputSize)
	m.metrics.RecordSamplingRequest(ctx, operation, success, duration, contentSize, responseSize, 0, "", 0, oldValidation)
}

func (m *metricsAdapter) RecordStreamingRequest(ctx context.Context, operation string, success bool, duration time.Duration, totalTokens int) {
	// The old metrics doesn't have streaming support, so we'll record as a regular request
	m.metrics.RecordSamplingRequest(ctx, operation, success, duration, 0, totalTokens, totalTokens, "stream", totalTokens, ValidationResult{IsValid: success})
}
