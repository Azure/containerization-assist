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

	// SagaIDKey stores the unique identifier for a saga transaction
	SagaIDKey contextKey = "saga_id"

	// SagaExecutionKey stores the saga execution context for compensation
	SagaExecutionKey contextKey = "saga_execution"

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

// WithSagaID adds a saga ID to the context
func WithSagaID(ctx context.Context, sagaID string) context.Context {
	return context.WithValue(ctx, SagaIDKey, sagaID)
}

// GetSagaID retrieves the saga ID from context
func GetSagaID(ctx context.Context) (string, bool) {
	sagaID, ok := ctx.Value(SagaIDKey).(string)
	return sagaID, ok
}

// WithSagaExecution adds saga execution context
func WithSagaExecution(ctx context.Context, sagaExec interface{}) context.Context {
	return context.WithValue(ctx, SagaExecutionKey, sagaExec)
}

// GetSagaExecution retrieves the saga execution from context
func GetSagaExecution(ctx context.Context) (interface{}, bool) {
	sagaExec := ctx.Value(SagaExecutionKey)
	if sagaExec == nil {
		return nil, false
	}
	return sagaExec, true
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
