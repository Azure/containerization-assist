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

	// TraceIDKey stores distributed tracing information
	TraceIDKey contextKey = "trace_id"
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

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// GetTraceID retrieves the trace ID from context
func GetTraceID(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(TraceIDKey).(string)
	return traceID, ok
}
