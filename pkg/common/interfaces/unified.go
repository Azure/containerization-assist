package interfaces

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

// UnifiedValidator consolidates all validation interfaces
// Replaces: 25+ validator interfaces → 1 interface (96% reduction)
type UnifiedValidator interface {
	// Input validation for tools
	ValidateInput(ctx context.Context, toolName string, input api.ToolInput) error

	// Output validation for tools
	ValidateOutput(ctx context.Context, toolName string, output api.ToolOutput) error

	// Configuration validation
	ValidateConfig(ctx context.Context, config interface{}) error

	// Schema validation
	ValidateSchema(ctx context.Context, schema interface{}, data interface{}) error

	// Health validation
	ValidateHealth(ctx context.Context) []ValidationResult
}

// UnifiedObservability consolidates all observability interfaces from telemetry package
// Replaces: 12+ observability interfaces → 1 interface (92% reduction)
type UnifiedObservability interface {
	// Structured logging with context
	Logger(ctx context.Context) StructuredLogger

	// Metrics collection and recording
	RecordMetric(ctx context.Context, name string, value float64, labels map[string]string)
	RecordToolExecution(ctx context.Context, toolName string, duration time.Duration, success bool)

	// Distributed tracing
	StartSpan(ctx context.Context, operationName string) (context.Context, TracingSpan)

	// Performance monitoring
	RecordP95Violation(ctx context.Context, toolName string, actual, target time.Duration)
	GetP95Target() time.Duration

	// Health and status
	Health() ObservabilityHealth
	Shutdown(ctx context.Context) error
}

// Supporting types for the interfaces

// ValidationResult represents a validation result
type ValidationResult struct {
	Valid    bool                   `json:"valid"`
	Message  string                 `json:"message"`
	Severity string                 `json:"severity"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// StructuredLogger provides structured logging capabilities
type StructuredLogger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	WithFields(fields map[string]interface{}) StructuredLogger
	WithError(err error) StructuredLogger
}

// TracingSpan represents a distributed tracing span
type TracingSpan interface {
	SetAttribute(key string, value interface{})
	SetStatus(code StatusCode, message string)
	RecordError(err error)
	End()
}

// StatusCode represents span status codes
type StatusCode int

const (
	StatusOK StatusCode = iota
	StatusError
)

// ObservabilityHealth represents the health of observability systems
type ObservabilityHealth struct {
	LoggingHealthy bool    `json:"logging_healthy"`
	MetricsHealthy bool    `json:"metrics_healthy"`
	TracingHealthy bool    `json:"tracing_healthy"`
	ErrorRate      float64 `json:"error_rate"`
}

// Common capability strings for dynamic discovery
const (
	// Validator capabilities
	CapabilityValidation       = "validation"
	CapabilitySchemaValidation = "schema_validation"
	CapabilityBusinessRules    = "business_rules"
)
