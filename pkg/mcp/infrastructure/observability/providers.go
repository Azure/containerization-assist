// Package observability provides unified dependency injection for monitoring and observability services
package observability

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/health"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/metrics"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/tracing"
	"github.com/google/wire"
)

// ObservabilityProviders provides all observability domain dependencies
var ObservabilityProviders = wire.NewSet(
	// Tracing
	tracing.NewTracerAdapter,

	// Health monitoring
	health.NewMonitor,

	// Metrics
	ProvideMetricsProvider,
)

// ProvideMetricsProvider creates a new metrics provider with default configuration
func ProvideMetricsProvider(logger *slog.Logger) (*metrics.MetricsProvider, error) {
	config := metrics.DefaultConfig()
	return metrics.NewMetricsProvider(config, logger)
}
