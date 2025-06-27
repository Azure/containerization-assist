package observability

import (
	"context"
	"fmt"
)

// Config holds observability configuration
type Config struct {
	// Enable observability features
	Enabled bool

	// Metrics configuration
	EnableMetrics bool
	MetricsPort   int
	MetricsPath   string

	// Tracing configuration
	EnableTracing   bool
	TracingEndpoint string
	TracingHeaders  map[string]string
	ServiceName     string
	ServiceVersion  string
	Environment     string
	SampleRate      float64
}

// DefaultConfig returns default observability configuration
func DefaultConfig() Config {
	return Config{
		Enabled:         false, // Disabled by default for minimal footprint
		EnableMetrics:   false,
		MetricsPort:     9090,
		MetricsPath:     "/metrics",
		EnableTracing:   false,
		TracingEndpoint: "",
		ServiceName:     "container-kit-mcp",
		ServiceVersion:  "dev",
		Environment:     "development",
		SampleRate:      1.0,
	}
}

// NewObservabilityManager creates an observability manager based on configuration
func NewObservabilityManager(config Config) (ObservabilityManager, error) {
	if !config.Enabled {
		return NewNoOpObservabilityManager(), nil
	}

	// If observability is enabled, create the full implementation
	// This would typically import the prometheus/otel packages only when needed
	return newFullObservabilityManager(config)
}

// newFullObservabilityManager creates a full observability manager
// This function will be implemented in a separate file with build tags
func newFullObservabilityManager(config Config) (ObservabilityManager, error) {
	// For now, return a minimal implementation
	// In a full build with observability enabled, this would create
	// the prometheus metrics collector and otel tracing provider
	return &MinimalObservabilityManager{
		config:  config,
		metrics: &NoOpMetricsCollector{},
		tracing: &NoOpTracingProvider{},
	}, nil
}

// MinimalObservabilityManager is a lightweight implementation
type MinimalObservabilityManager struct {
	config  Config
	metrics ObservabilityMetricsCollector
	tracing TracingProvider
}

func (m *MinimalObservabilityManager) Metrics() ObservabilityMetricsCollector {
	return m.metrics
}

func (m *MinimalObservabilityManager) Tracing() TracingProvider {
	return m.tracing
}

func (m *MinimalObservabilityManager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		return nil
	}
	// In a full implementation, this would start metrics server and tracing
	return nil
}

func (m *MinimalObservabilityManager) Stop(ctx context.Context) error {
	return nil
}

func (m *MinimalObservabilityManager) IsEnabled() bool {
	return m.config.Enabled
}

// Helper functions for backward compatibility

// GetMetricsCollector returns a metrics collector based on configuration
func GetMetricsCollector(enabled bool) ObservabilityMetricsCollector {
	if !enabled {
		return &NoOpMetricsCollector{}
	}
	// In full builds, would return prometheus collector
	return &NoOpMetricsCollector{}
}

// GetTracingProvider returns a tracing provider based on configuration
func GetTracingProvider(enabled bool, config Config) TracingProvider {
	if !enabled {
		return &NoOpTracingProvider{}
	}
	// In full builds, would return otel provider
	return &NoOpTracingProvider{}
}

// InitializeObservability sets up observability with the given configuration
func InitializeObservability(config Config) (ObservabilityManager, error) {
	manager, err := NewObservabilityManager(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create observability manager: %w", err)
	}

	// Start the manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start observability manager: %w", err)
	}

	return manager, nil
}
