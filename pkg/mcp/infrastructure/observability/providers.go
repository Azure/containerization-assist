// Package observability provides unified dependency injection for monitoring and observability services
package observability

import (
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/health"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/tracing"
	"github.com/google/wire"
)

// ObservabilityProviders provides all observability domain dependencies
var ObservabilityProviders = wire.NewSet(
	// Core observability
	ProvideUnifiedObserver,
	ProvideUnifiedLogger,

	// Tracing
	tracing.NewTracerAdapter,

	// Health monitoring
	health.NewMonitor,

	// Configuration
	ProvideObservabilityConfig,
)

// ProvideUnifiedObserver creates a unified observer instance
func ProvideUnifiedObserver(logger *slog.Logger) Observer {
	config := DefaultObserverConfig()
	return NewUnifiedObserver(logger, config)
}

// ProvideUnifiedLogger creates a unified logger instance with default enrichers
func ProvideUnifiedLogger(observer Observer, logger *slog.Logger) *UnifiedLogger {
	config := DefaultLoggerConfig()
	unifiedLogger := NewUnifiedLogger(observer, logger, config)

	// Add default enrichers for comprehensive logging
	unifiedLogger.AddEnricher(NewSystemEnricher(true, true, true))
	unifiedLogger.AddEnricher(NewPerformanceEnricher(true, false, 10))
	unifiedLogger.AddEnricher(NewTimestampEnricher(true, true, true))
	unifiedLogger.AddEnricher(NewContextEnricher([]string{
		"user_id", "session_id", "workflow_id", "operation_id", "request_id",
	}))
	unifiedLogger.AddEnricher(NewSecurityEnricher(true, true, true, []string{
		"password", "secret", "token", "key", "api_key", "auth_token",
	}))
	unifiedLogger.AddEnricher(NewBusinessEnricher(true, true, map[string]string{
		"deploy":      "deployment",
		"container":   "container_management",
		"workflow":    "workflow_execution",
		"user":        "user_activity",
		"auth":        "authentication",
		"error":       "error_event",
		"performance": "performance_event",
	}))

	return unifiedLogger
}

// Note: ProvidePerformanceMonitor is defined in the performance package
// to avoid import cycles. The performance package provides its own provider function.

// ProvideObservabilityConfig creates observability configuration
func ProvideObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		EnableUnifiedLogging:        true,
		EnableMetricExtraction:      true,
		EnablePerformanceMonitoring: true,
		EnableTraceCorrelation:      true,
		EnableErrorAggregation:      true,
		LogLevel:                    slog.LevelInfo,
		SamplingRate:                1.0,
		RetentionPeriod:             DefaultObserverConfig().RetentionPeriod,
		MaxConcurrentObservers:      10,
		BufferSize:                  5000,
		FlushInterval:               time.Minute * 5,
	}
}

// ObservabilityConfig provides configuration for the entire observability system
type ObservabilityConfig struct {
	// Feature flags
	EnableUnifiedLogging        bool
	EnableMetricExtraction      bool
	EnablePerformanceMonitoring bool
	EnableTraceCorrelation      bool
	EnableErrorAggregation      bool

	// Logging configuration
	LogLevel        slog.Level
	SamplingRate    float64
	RetentionPeriod time.Duration

	// Performance tuning
	MaxConcurrentObservers int
	BufferSize             int
	FlushInterval          time.Duration

	// Integration settings
	MetricsBackend string
	TracingBackend string
	LoggingBackend string
}

// Environment-specific configurations

// DevelopmentObservabilityConfig returns configuration optimized for development
func DevelopmentObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		EnableUnifiedLogging:        true,
		EnableMetricExtraction:      true,
		EnablePerformanceMonitoring: true,
		EnableTraceCorrelation:      true,
		EnableErrorAggregation:      true,
		LogLevel:                    slog.LevelDebug,
		SamplingRate:                1.0, // Log everything in development
		RetentionPeriod:             time.Hour * 2,
		MaxConcurrentObservers:      5,
		BufferSize:                  1000,
		FlushInterval:               time.Second * 10,
	}
}

// ProductionObservabilityConfig returns configuration optimized for production
func ProductionObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		EnableUnifiedLogging:        true,
		EnableMetricExtraction:      true,
		EnablePerformanceMonitoring: true,
		EnableTraceCorrelation:      true,
		EnableErrorAggregation:      true,
		LogLevel:                    slog.LevelInfo,
		SamplingRate:                0.1, // Sample 10% in production
		RetentionPeriod:             time.Hour * 24,
		MaxConcurrentObservers:      20,
		BufferSize:                  10000,
		FlushInterval:               time.Minute * 5,
	}
}

// TestObservabilityConfig returns configuration optimized for testing
func TestObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		EnableUnifiedLogging:        true,
		EnableMetricExtraction:      false, // Disable for faster tests
		EnablePerformanceMonitoring: false, // Disable for faster tests
		EnableTraceCorrelation:      false, // Disable for faster tests
		EnableErrorAggregation:      true,
		LogLevel:                    slog.LevelError, // Only errors in tests
		SamplingRate:                1.0,
		RetentionPeriod:             time.Minute * 5,
		MaxConcurrentObservers:      2,
		BufferSize:                  100,
		FlushInterval:               time.Second,
	}
}
