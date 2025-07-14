// Package observability provides unified dependency injection for monitoring and observability services
package observability

import (
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/health"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/tracing"
	"github.com/google/wire"
)

// ObservabilityProviders provides all observability domain dependencies
var ObservabilityProviders = wire.NewSet(
	// Tracing
	tracing.NewTracerAdapter,

	// Health monitoring
	health.NewMonitor,
)
