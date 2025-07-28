// Package workflow provides typed context keys for workflow operations.
// Using typed context keys prevents collisions and catches typos at compile time.
package workflow

import (
	"context"
)

// contextKey prevents external collisions when storing values in context.
// This unexported type ensures that only this package can create context keys,
// preventing accidental conflicts with other packages.
type contextKey string

// Context key constants for workflow operations
const (
	// WorkflowIDKey stores the unique identifier for a workflow execution
	WorkflowIDKey contextKey = "workflow_id"

	// TraceIDKey stores correlation information (retained for API compatibility)
	TraceIDKey contextKey = "trace_id"

	// RetryAttemptKey stores the current retry attempt number
	RetryAttemptKey contextKey = "retry_attempt"

	// OrchestratorConfigKey stores the orchestrator configuration
	OrchestratorConfigKey contextKey = "orchestrator_config"
)

// WithWorkflowID adds a workflow ID to the context
func WithWorkflowID(ctx context.Context, workflowID string) context.Context {
	return context.WithValue(ctx, WorkflowIDKey, workflowID)
}

// GetWorkflowID retrieves the workflow ID from context
func GetWorkflowID(ctx context.Context) (string, bool) {
	workflowID, ok := ctx.Value(WorkflowIDKey).(string)
	return workflowID, ok
}

// WithTraceID adds a trace ID to the context (retained for API compatibility)
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// GetTraceID retrieves the trace ID from context (retained for API compatibility)
func GetTraceID(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(TraceIDKey).(string)
	return traceID, ok
}

// WithRetryAttempt adds a retry attempt number to the context
func WithRetryAttempt(ctx context.Context, attempt int) context.Context {
	return context.WithValue(ctx, RetryAttemptKey, attempt)
}

// GetRetryAttempt retrieves the retry attempt number from context
func GetRetryAttempt(ctx context.Context) (int, bool) {
	attempt, ok := ctx.Value(RetryAttemptKey).(int)
	return attempt, ok
}

// WithOrchestratorConfig adds orchestrator configuration to the context
func WithOrchestratorConfig(ctx context.Context, config OrchestratorConfig) context.Context {
	return context.WithValue(ctx, OrchestratorConfigKey, config)
}

// GetOrchestratorConfig retrieves the orchestrator configuration from context
func GetOrchestratorConfig(ctx context.Context) (OrchestratorConfig, bool) {
	config, ok := ctx.Value(OrchestratorConfigKey).(OrchestratorConfig)
	return config, ok
}
