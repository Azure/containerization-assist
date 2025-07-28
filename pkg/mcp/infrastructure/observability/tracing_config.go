// Package observability provides unified monitoring and health infrastructure
// for the MCP components. It consolidates health checks and logging enrichment
// into a single coherent package.
package observability

import (
	"context"
	"time"
)

// Config holds configuration for observability features
type Config struct {
	// ServiceName identifies the service
	ServiceName string

	// ServiceVersion identifies the service version
	ServiceVersion string

	// Environment identifies the deployment environment
	Environment string

	// ExportTimeout for operations
	ExportTimeout time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		ServiceName:    "container-kit-mcp",
		ServiceVersion: "dev",
		Environment:    "development",
		ExportTimeout:  30 * time.Second,
	}
}

// InitializeTracing is a no-op function that maintains API compatibility
func InitializeTracing(ctx context.Context, config Config) error {
	// No-op implementation, telemetry has been removed
	return nil
}

// Shutdown is a no-op function that maintains API compatibility
func Shutdown(ctx context.Context) error {
	// No-op implementation, telemetry has been removed
	return nil
}

// GetTracer returns a no-op implementation
func GetTracer(name string) interface{} {
	return nil
}

// SpanFromContext returns nil for compatibility
func SpanFromContext(ctx context.Context) interface{} {
	return nil
}

// StartSpan returns the original context and nil span for compatibility
func StartSpan(ctx context.Context, name string, opts ...interface{}) (context.Context, interface{}) {
	return ctx, nil
}
